package sslmonitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ocsp"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// CheckResult is the full output of a TLS check.
type CheckResult struct {
	Domain       string    `json:"domain"`
	Port         int       `json:"port"`
	CheckedAt    time.Time `json:"checked_at"`
	Status       string    `json:"status"` // valid, expiring_soon, expired, error
	Error        string    `json:"error,omitempty"`

	// Certificate
	Issuer        string     `json:"issuer"`
	Subject       string     `json:"subject"`
	CertExpiresAt *time.Time `json:"cert_expires_at"`
	DaysRemaining int        `json:"days_remaining"`

	// Chain validation
	ChainValid *bool  `json:"chain_valid,omitempty"`
	ChainError string `json:"chain_error,omitempty"`

	// Cipher
	CipherGrade string `json:"cipher_grade"`
	CipherError string `json:"cipher_error,omitempty"`
	TLSVersion  string `json:"tls_version,omitempty"`
	CipherName  string `json:"cipher_name,omitempty"`

	// OCSP
	OCSPStatus string `json:"ocsp_status"`
	OCSPError  string `json:"ocsp_error,omitempty"`

	// SAN
	SANNames    []string `json:"san_names,omitempty"`
	SANMismatch bool     `json:"san_mismatch"`
}

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	StatusValid        = "valid"
	StatusExpiringSoon = "expiring_soon"
	StatusExpired      = "expired"
	StatusError        = "error"

	GradeAPlus = "A+"
	GradeA     = "A"
	GradeB     = "B"
	GradeC     = "C"
	GradeD     = "D"
	GradeF     = "F"

	OCSPGood    = "good"
	OCSPRevoked = "revoked"
	OCSPUnknown = "unknown"

	ExpiringSoonDays = 30
)

// ─── TLS Check Engine ────────────────────────────────────────────────────────

// Check performs a full TLS certificate check against the given domain:port.
func Check(ctx context.Context, domain string, port int) *CheckResult {
	addr := net.JoinHostPort(domain, fmt.Sprintf("%d", port))

	result := &CheckResult{
		Domain:    domain,
		Port:      port,
		CheckedAt: time.Now().UTC(),
	}

	// ── Dial with TLS ──────────────────────────────────────────────────────
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		result.Status = StatusError
		result.Error = fmt.Sprintf("TLS dial failed: %v", err)
		result.CipherGrade = GradeF
		result.OCSPStatus = OCSPUnknown
		return result
	}
	defer conn.Close()

	cs := conn.ConnectionState()

	if len(cs.PeerCertificates) == 0 {
		result.Status = StatusError
		result.Error = "no peer certificates returned"
		result.CipherGrade = GradeF
		result.OCSPStatus = OCSPUnknown
		return result
	}

	leaf := cs.PeerCertificates[0]
	chain := cs.PeerCertificates

	// ── Basic cert info ────────────────────────────────────────────────────
	result.Subject = leaf.Subject.String()
	if leaf.Issuer.CommonName != "" {
		result.Issuer = leaf.Issuer.CommonName
	} else if len(leaf.Issuer.Organization) > 0 {
		result.Issuer = leaf.Issuer.Organization[0]
	} else {
		result.Issuer = leaf.Issuer.String()
	}

	expiresAt := leaf.NotAfter
	result.CertExpiresAt = &expiresAt
	result.DaysRemaining = int(time.Until(expiresAt).Hours() / 24)

	now := time.Now()
	switch {
	case now.After(expiresAt):
		result.Status = StatusExpired
	case expiresAt.Sub(now) < ExpiringSoonDays*24*time.Hour:
		result.Status = StatusExpiringSoon
	default:
		result.Status = StatusValid
	}

	// ── SAN names ──────────────────────────────────────────────────────────
	result.SANNames = leaf.DNSNames
	if len(leaf.DNSNames) == 0 && leaf.Subject.CommonName != "" {
		result.SANNames = []string{leaf.Subject.CommonName}
	}

	normalizedDomain := strings.TrimSuffix(strings.ToLower(domain), ".")
	matched := false
	for _, san := range leaf.DNSNames {
		if matchSAN(normalizedDomain, strings.ToLower(san)) {
			matched = true
			break
		}
	}
	if !matched {
		result.SANMismatch = true
		result.ChainError = fmt.Sprintf("domain %q not covered by certificate SAN names", domain)
	}

	// ── Chain validation ───────────────────────────────────────────────────
	chainValid, chainErr := validateChain(chain)
	result.ChainValid = &chainValid
	if chainErr != "" {
		if result.ChainError != "" {
			result.ChainError += "; " + chainErr
		} else {
			result.ChainError = chainErr
		}
	}

	// ── Cipher grade ───────────────────────────────────────────────────────
	result.TLSVersion = tlsVersionString(cs.Version)
	result.CipherName = tls.CipherSuiteName(cs.CipherSuite)
	result.CipherGrade = gradeTLS(cs.Version, cs.CipherSuite)

	// ── OCSP check ─────────────────────────────────────────────────────────
	if len(cs.OCSPResponse) > 0 {
		result.OCSPStatus = checkOCSPStapled(cs.OCSPResponse, leaf)
	} else {
		result.OCSPStatus, result.OCSPError = checkOCSPOnline(leaf, chain)
	}

	return result
}

// ─── Chain Validation ────────────────────────────────────────────────────────

func validateChain(chain []*x509.Certificate) (bool, string) {
	if len(chain) == 0 {
		return false, "empty certificate chain"
	}

	leaf := chain[0]
	now := time.Now()

	if now.After(leaf.NotAfter) {
		return false, "certificate has expired"
	}
	if now.Before(leaf.NotBefore) {
		return false, "certificate is not yet valid"
	}

	// Check issuer-subject relationships + signatures
	for i := 0; i < len(chain)-1; i++ {
		child := chain[i]
		parent := chain[i+1]
		if child.Issuer.CommonName != "" && parent.Subject.CommonName != "" &&
			child.Issuer.CommonName != parent.Subject.CommonName {
			return false, fmt.Sprintf("chain break at link %d: issuer %q != subject %q",
				i, child.Issuer.CommonName, parent.Subject.CommonName)
		}
		if err := child.CheckSignatureFrom(parent); err != nil {
			return false, fmt.Sprintf("signature verification failed at link %d: %v", i, err)
		}
	}

	// Build intermediate pool and try system root verification
	intermediates := x509.NewCertPool()
	for i := 1; i < len(chain); i++ {
		intermediates.AddCert(chain[i])
	}

	rootPool, err := x509.SystemCertPool()
	if err == nil && rootPool != nil {
		_, verifyErr := leaf.Verify(x509.VerifyOptions{
			Intermediates: intermediates,
			Roots:         rootPool,
			CurrentTime:   now,
		})
		if verifyErr != nil {
			return false, fmt.Sprintf("full chain verification failed: %v", verifyErr)
		}
		return true, ""
	}

	// Without system roots, check if last cert is self-signed (root)
	last := chain[len(chain)-1]
	if last.IsCA && last.Subject.CommonName == last.Issuer.CommonName && last.Subject.CommonName != "" {
		return true, "root CA is self-signed (expected for Let's Encrypt, etc.)"
	}

	return true, "chain structurally valid"
}

// ─── TLS Version String ──────────────────────────────────────────────────────

func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("TLS 0x%04x", version)
	}
}

// ─── Cipher Grade ────────────────────────────────────────────────────────────

func gradeTLS(version uint16, cipherSuite uint16) string {
	switch version {
	case tls.VersionTLS13:
		return GradeAPlus
	case tls.VersionTLS12:
		name := tls.CipherSuiteName(cipherSuite)
		if strings.Contains(name, "GCM") || strings.Contains(name, "CCM") || strings.Contains(name, "CHACHA20") {
			return GradeA
		}
		return GradeB
	case tls.VersionTLS11:
		return GradeC
	case tls.VersionTLS10:
		return GradeD
	default:
		return GradeF
	}
}

// ─── OCSP Check ──────────────────────────────────────────────────────────────

func checkOCSPStapled(ocspBytes []byte, leaf *x509.Certificate) string {
	resp, err := ocsp.ParseResponse(ocspBytes, nil)
	if err != nil {
		return OCSPUnknown
	}
	switch resp.Status {
	case ocsp.Good:
		return OCSPGood
	case ocsp.Revoked:
		return OCSPRevoked
	default:
		return OCSPUnknown
	}
}

func checkOCSPOnline(leaf *x509.Certificate, chain []*x509.Certificate) (string, string) {
	if len(leaf.OCSPServer) == 0 {
		return OCSPUnknown, "no OCSP server URL in certificate"
	}

	var issuer *x509.Certificate
	if len(chain) > 1 {
		issuer = chain[1]
	} else {
		return OCSPUnknown, "no issuer certificate in chain for OCSP verification"
	}

	ocspReqBytes, err := ocsp.CreateRequest(leaf, issuer, &ocsp.RequestOptions{})
	if err != nil {
		return OCSPUnknown, fmt.Sprintf("failed to create OCSP request: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, server := range leaf.OCSPServer {
		if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
			continue
		}

		resp, err := client.Post(server, "application/ocsp-request", bytes.NewReader(ocspReqBytes))
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			continue
		}

		ocspResp, err := ocsp.ParseResponse(body, nil)
		if err != nil {
			continue
		}

		switch ocspResp.Status {
		case ocsp.Good:
			return OCSPGood, ""
		case ocsp.Revoked:
			return OCSPRevoked, "certificate has been revoked"
		default:
			return OCSPUnknown, "OCSP response status unknown"
		}
	}

	return OCSPUnknown, "all OCSP servers unreachable"
}

// ─── SAN Matching ────────────────────────────────────────────────────────────

func matchSAN(domain, pattern string) bool {
	if pattern == domain {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:]
		return strings.HasSuffix(domain, suffix)
	}
	return false
}
