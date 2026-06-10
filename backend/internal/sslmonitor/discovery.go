package sslmonitor

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/edsuwarna/anjungan/internal/common/db"
	"github.com/edsuwarna/anjungan/internal/common/model"
)

// Discoverer handles server-side SSL certificate discovery via SSH.
type Discoverer struct {
	repo *db.Repository
}

// NewDiscoverer creates a new discovery engine.
func NewDiscoverer(repo *db.Repository) *Discoverer {
	return &Discoverer{repo: repo}
}

// Discover runs auto-detection or a specific provider on a server.
func (d *Discoverer) Discover(ctx context.Context, server *model.Server, provider string) (*model.SSLDiscoveryResponse, error) {
	client, err := sshConnect(server)
	if err != nil {
		return nil, fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()

	if provider == "auto" || provider == "" {
		providers := []string{"traefik", "nginx", "caddy", "letsencrypt", "filesystem"}
		for _, p := range providers {
			result, err := d.discoverByType(client, p)
			if err == nil && len(result.Domains) > 0 {
				return result, nil
			}
		}
		return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
	}

	return d.discoverByType(client, provider)
}

func (d *Discoverer) discoverByType(client *ssh.Client, provider string) (*model.SSLDiscoveryResponse, error) {
	switch provider {
	case "traefik":
		return d.discoverTraefik(client)
	case "nginx":
		return d.discoverNginx(client)
	case "caddy":
		return d.discoverCaddy(client)
	case "letsencrypt":
		return d.discoverLetsEncrypt(client)
	case "filesystem":
		return d.discoverFilesystem(client)
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// ─── SSH helpers ────────────────────────────────────────────────────────────

func sshConnect(s *model.Server) (*ssh.Client, error) {
	addr := net.JoinHostPort(s.Host, fmt.Sprintf("%d", s.Port))

	var auth ssh.AuthMethod
	switch s.SSHAuthType {
	case "password":
		auth = ssh.Password(s.SSHPassword)
	case "key":
		if s.SSHKey == "" {
			return nil, fmt.Errorf("SSH key is empty for server %s", s.Name)
		}
		signer, err := ssh.ParsePrivateKey([]byte(s.SSHKey))
		if err != nil {
			return nil, fmt.Errorf("parse SSH key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", s.SSHAuthType)
	}

	sshCfg := &ssh.ClientConfig{
		User:            s.SSHUser,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial: %w", err)
	}
	return client, nil
}

func sshRun(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("command: %w — output: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// readRemoteFile reads a file from the remote server via SSH.
func readRemoteFile(client *ssh.Client, path string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}
	defer session.Close()

	output, err := session.Output(fmt.Sprintf("cat %s 2>/dev/null", path))
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	return output, nil
}

// ─── Certificate parsing ───────────────────────────────────────────────────

// parseCertPEM parses a PEM-encoded certificate and returns expiry, issuer, SANs.
func parseCertPEM(pemData string) (time.Time, string, []string) {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return time.Time{}, "", nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, "", nil
	}

	issuer := cert.Issuer.CommonName
	if issuer == "" && len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}

	return cert.NotAfter, issuer, cert.DNSNames
}

// extractDomainFromPath extracts the domain from a certificate file path.
// Handles LetsEncrypt pattern: /etc/letsencrypt/live/<domain>/fullchain.pem
// and generic patterns.
func extractDomainFromPath(path string) string {
	// LetsEncrypt pattern: /etc/letsencrypt/live/<domain>/...
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "live" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	// Fallback: use filename without extension
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".pem")
		name = strings.TrimSuffix(name, ".crt")
		name = strings.TrimSuffix(name, ".cert")
		return name
	}
	return path
}

// ─── Provider implementations ──────────────────────────────────────────────

// Traefik — parse acme.json
func (d *Discoverer) discoverTraefik(client *ssh.Client) (*model.SSLDiscoveryResponse, error) {
	paths := []string{
		"/etc/traefik/acme.json",
		"/traefik/acme.json",
		"/var/lib/traefik/acme.json",
		"/opt/traefik/acme.json",
		"/etc/traefik/acme/acme.json",
	}

	// Try each path
	var data []byte
	var readErr error
	for _, path := range paths {
		data, readErr = readRemoteFile(client, path)
		if readErr == nil && len(data) > 0 {
			break
		}
	}
	if len(data) == 0 {
		return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
	}

	// Try Docker container for traefik
	if len(data) == 0 {
		out, err := sshRun(client, `docker ps --format '{{.Names}}' 2>/dev/null | grep -i traefik | head -1`)
		if err == nil && out != "" {
			containerName := strings.TrimSpace(out)
			cmd := fmt.Sprintf("docker exec %s cat /acme.json 2>/dev/null", containerName)
			out2, err2 := sshRun(client, cmd)
			if err2 == nil && out2 != "" {
				data = []byte(out2)
			}
		}
	}

	if len(data) == 0 {
		return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
	}

	// Try both Traefik v2 and v3 acme.json format
	var acmeData struct {
		Certificates []struct {
			Domain struct {
				Main string   `json:"Main"`
				SANs []string `json:"SANs"`
			} `json:"Domain"`
			Certificate string `json:"Certificate"`
		} `json:"Certificates"`
	}
	if err := json.Unmarshal(data, &acmeData); err != nil || len(acmeData.Certificates) == 0 {
		// Try v2 nested structure: {"acme": {...}, "resolvers": {"default": {"acme": {...}}}}
		var nested struct {
			Resolvers map[string]struct {
				Acme struct {
					Certificates []struct {
						Domain struct {
							Main string   `json:"Main"`
							SANs []string `json:"SANs"`
						} `json:"Domain"`
						Certificate string `json:"Certificate"`
					} `json:"Certificates"`
				} `json:"acme"`
			} `json:"resolvers"`
		}
		if err2 := json.Unmarshal(data, &nested); err2 != nil {
			return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
		}
		for _, resolver := range nested.Resolvers {
			acmeData.Certificates = append(acmeData.Certificates, resolver.Acme.Certificates...)
		}
	}

	var results []model.SSLDiscoveryResult
	seen := map[string]bool{}
	for _, cert := range acmeData.Certificates {
		if cert.Domain.Main == "" || seen[cert.Domain.Main] {
			continue
		}
		seen[cert.Domain.Main] = true

		expiry, issuer, sans := parseCertPEM(cert.Certificate)
		allSans := append([]string{cert.Domain.Main}, cert.Domain.SANs...)
		if len(sans) > 0 {
			allSans = sans
		}

		results = append(results, model.SSLDiscoveryResult{
			Domain:         cert.Domain.Main,
			Port:           443,
			DisplayName:    cert.Domain.Main,
			CertExpiresAt:  expiry.Format(time.RFC3339),
			Issuer:         issuer,
			SANNames:       allSans,
			SourceProvider: "traefik",
		})
	}

	return &model.SSLDiscoveryResponse{Domains: results}, nil
}

// Nginx — scan config files for ssl_certificate directives
func (d *Discoverer) discoverNginx(client *ssh.Client) (*model.SSLDiscoveryResponse, error) {
	// Try nginx -T first (requires nginx binary)
	out, err := sshRun(client, `nginx -T 2>/dev/null | grep -oP 'ssl_certificate\s+\K\S+' | sed 's/;$//' | sort -u`)
	if err != nil || out == "" {
		// Fallback: grep config files directly
		out, err = sshRun(client, `grep -roP 'ssl_certificate\s+\K\S+' /etc/nginx/ 2>/dev/null | sed 's/;$//' | sort -u`)
	}
	if out == "" {
		return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
	}

	lines := strings.Split(out, "\n")
	var results []model.SSLDiscoveryResult
	seen := map[string]bool{}

	for _, certPath := range lines {
		certPath = strings.TrimSpace(certPath)
		if certPath == "" || seen[certPath] {
			continue
		}
		seen[certPath] = true

		domain := extractDomainFromPath(certPath)
		certData, err := readRemoteFile(client, certPath)
		if err != nil {
			continue
		}
		expiry, issuer, sans := parseCertPEM(string(certData))

		results = append(results, model.SSLDiscoveryResult{
			Domain:         domain,
			Port:           443,
			DisplayName:    domain,
			CertExpiresAt:  expiry.Format(time.RFC3339),
			Issuer:         issuer,
			SANNames:       sans,
			CertPath:       certPath,
			SourceProvider: "nginx",
		})
	}

	return &model.SSLDiscoveryResponse{Domains: results}, nil
}

// Caddy — scan Caddy storage or config
func (d *Discoverer) discoverCaddy(client *ssh.Client) (*model.SSLDiscoveryResponse, error) {
	// Caddy v2+ stores certificates in /var/lib/caddy/.local/share/certificates/
	// or ~/.local/share/certificates/
	paths := []string{
		"/var/lib/caddy/.local/share/certificates",
		"/root/.local/share/certificates",
		"/home/*/.local/share/certificates",
	}

	// Check if caddy storage.json exists
	caddyPaths := []string{
		"/var/lib/caddy/storage.json",
		"/etc/caddy/storage.json",
		"~/.config/caddy/storage.json",
	}

	var data []byte
	for _, path := range caddyPaths {
		d, err := readRemoteFile(client, path)
		if err == nil && len(d) > 0 {
			data = d
			break
		}
	}

	if len(data) > 0 {
		var storage struct {
			Certificates map[string]struct {
				Cert string `json:"cert"`
			} `json:"certificates"`
		}
		if err := json.Unmarshal(data, &storage); err == nil {
			var results []model.SSLDiscoveryResult
			for domain, certInfo := range storage.Certificates {
				expiry, issuer, sans := parseCertPEM(certInfo.Cert)
				results = append(results, model.SSLDiscoveryResult{
					Domain:         domain,
					Port:           443,
					DisplayName:    domain,
					CertExpiresAt:  expiry.Format(time.RFC3339),
					Issuer:         issuer,
					SANNames:       sans,
					SourceProvider: "caddy",
				})
			}
			if len(results) > 0 {
				return &model.SSLDiscoveryResponse{Domains: results}, nil
			}
		}
	}

	// Fallback: scan for PEM files in caddy directories
	for _, base := range paths {
		out, err := sshRun(client, fmt.Sprintf(`find %s -name "*.pem" -o -name "*.crt" 2>/dev/null | head -50`, base))
		if err != nil || out == "" {
			continue
		}
		files := strings.Split(out, "\n")
		var results []model.SSLDiscoveryResult
		seen := map[string]bool{}

		for _, f := range files {
			f = strings.TrimSpace(f)
			if f == "" || seen[f] {
				continue
			}
			seen[f] = true
			domain := extractDomainFromPath(f)
			certData, err := readRemoteFile(client, f)
			if err != nil {
				continue
			}
			expiry, issuer, sans := parseCertPEM(string(certData))
			results = append(results, model.SSLDiscoveryResult{
				Domain:         domain,
				Port:           443,
				DisplayName:    domain,
				CertExpiresAt:  expiry.Format(time.RFC3339),
				Issuer:         issuer,
				SANNames:       sans,
				CertPath:       f,
				SourceProvider: "caddy",
			})
		}
		if len(results) > 0 {
			return &model.SSLDiscoveryResponse{Domains: results}, nil
		}
	}

	return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
}

// LetsEncrypt — scan /etc/letsencrypt/live/
func (d *Discoverer) discoverLetsEncrypt(client *ssh.Client) (*model.SSLDiscoveryResponse, error) {
	out, err := sshRun(client, `ls -d /etc/letsencrypt/live/*/ 2>/dev/null | sed 's|/etc/letsencrypt/live/||;s|/||'`)
	if err != nil || out == "" {
		return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
	}

	lines := strings.Split(out, "\n")
	var results []model.SSLDiscoveryResult
	seen := map[string]bool{}

	for _, domain := range lines {
		domain = strings.TrimSpace(domain)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true

		certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", domain)

		// Check if cert exists
		checkOut, _ := sshRun(client, fmt.Sprintf("test -f %s && echo EXISTS || echo NOT_FOUND", certPath))
		if strings.TrimSpace(checkOut) != "EXISTS" {
			continue
		}

		certData, err := readRemoteFile(client, certPath)
		if err != nil {
			continue
		}
		expiry, issuer, sans := parseCertPEM(string(certData))

		results = append(results, model.SSLDiscoveryResult{
			Domain:         domain,
			Port:           443,
			DisplayName:    domain,
			CertExpiresAt:  expiry.Format(time.RFC3339),
			Issuer:         issuer,
			SANNames:       sans,
			CertPath:       certPath,
			SourceProvider: "letsencrypt",
		})
	}

	return &model.SSLDiscoveryResponse{Domains: results}, nil
}

// Filesystem — generic scan for PEM files
func (d *Discoverer) discoverFilesystem(client *ssh.Client) (*model.SSLDiscoveryResponse, error) {
	// Scan common certificate directories
	dirs := []string{
		"/etc/ssl/certs",
		"/etc/pki/tls/certs",
		"/etc/certs",
		"/etc/ssl/private",
		"/etc/pki/tls/private",
	}

	// Only scan files, not symlinks, and filter for actual cert files (not CA bundles)
	// Exclude ca-certificates.crt and similar bundle files
	for _, dir := range dirs {
		out, err := sshRun(client, fmt.Sprintf(
			`find %s -maxdepth 1 -type f \( -name "*.pem" -o -name "*.crt" -o -name "*.cert" \) 2>/dev/null | head -20`,
			dir,
		))
		if err != nil || out == "" {
			continue
		}

		files := strings.Split(out, "\n")
		var results []model.SSLDiscoveryResult
		seen := map[string]bool{}

		for _, f := range files {
			f = strings.TrimSpace(f)
			if f == "" || seen[f] {
				continue
			}
			// Skip CA bundles
			if strings.Contains(f, "ca-certificates") || strings.Contains(f, "ca-bundle") {
				continue
			}
			seen[f] = true

			certData, err := readRemoteFile(client, f)
			if err != nil {
				continue
			}

			expiry, issuer, sans := parseCertPEM(string(certData))
			if expiry.IsZero() {
				continue
			}

			domain := extractDomainFromPath(f)
			if domain == "" {
				domain = fmt.Sprintf("cert-%d", len(results)+1)
			}

			results = append(results, model.SSLDiscoveryResult{
				Domain:         domain,
				Port:           443,
				DisplayName:    domain,
				CertExpiresAt:  expiry.Format(time.RFC3339),
				Issuer:         issuer,
				SANNames:       sans,
				CertPath:       f,
				SourceProvider: "discovered",
			})
		}

		if len(results) > 0 {
			return &model.SSLDiscoveryResponse{Domains: results}, nil
		}
	}

	return &model.SSLDiscoveryResponse{Domains: []model.SSLDiscoveryResult{}}, nil
}
