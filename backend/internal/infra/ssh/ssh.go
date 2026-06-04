package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Host       string
	Port       int
	User       string
	AuthType   string // "key" or "password"
	Key        string // PEM-encoded private key
	Password   string
	Timeout    time.Duration
}

// dial establishes an SSH connection to the server
func dial(ctx context.Context, cfg Config) (*ssh.Client, error) {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))

	var auth ssh.AuthMethod
	switch cfg.AuthType {
	case "password":
		auth = ssh.Password(cfg.Password)
	case "key":
		if cfg.Key == "" {
			return nil, fmt.Errorf("SSH key is empty")
		}
		signer, err := ssh.ParsePrivateKey([]byte(cfg.Key))
		if err != nil {
			return nil, fmt.Errorf("parse SSH key: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", cfg.AuthType)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial: %w", err)
	}

	return client, nil
}

// runCommand executes a command on an established SSH connection
func runCommand(client *ssh.Client, cmd string) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return output, fmt.Errorf("SSH command: %w", err)
	}

	return output, nil
}

// TestConnection attempts to SSH into the server and returns the server's hostname
func TestConnection(ctx context.Context, cfg Config) (string, error) {
	client, err := dial(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer client.Close()

	output, err := runCommand(client, "hostname")
	if err != nil {
		return "", fmt.Errorf("SSH command: %w", err)
	}

	hostname := string(output)
	if len(hostname) > 100 {
		hostname = hostname[:100]
	}

	return hostname, nil
}

// RunCommand executes a command on the remote server and returns stdout
func RunCommand(ctx context.Context, cfg Config, cmd string) (string, error) {
	client, err := dial(ctx, cfg)
	if err != nil {
		return "", err
	}
	defer client.Close()

	output, err := runCommand(client, cmd)
	if err != nil {
		return "", fmt.Errorf("SSH command: %w", err)
	}
	return string(output), nil
}

// NewTerminalSession creates an interactive SSH session with PTY and returns the client + session
// The caller is responsible for closing both client and session.
// stdin/stdout/stderr pipes are returned for I/O.
func NewTerminalSession(ctx context.Context, cfg Config, term string, rows, cols int) (*ssh.Client, *ssh.Session, io.Reader, io.WriteCloser, error) {
	client, err := dial(ctx, cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("SSH dial: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH session: %w", err)
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty(term, rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH PTY: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stderr pipe: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stdin pipe: %w", err)
	}

	// Start shell
	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH shell: %w", err)
	}

	// Merge stdout + stderr
	combined := io.MultiReader(stdout, stderr)

	return client, session, combined, stdin, nil
}

// NewContainerExecSession creates an interactive SSH session that runs `docker exec` into a container
// cmd is the full docker exec command to run (e.g. "docker exec -it myapp /bin/sh")
func NewContainerExecSession(ctx context.Context, cfg Config, cmd string, term string, rows, cols int) (*ssh.Client, *ssh.Session, io.Reader, io.WriteCloser, error) {
	client, err := dial(ctx, cfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("SSH dial: %w", err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH session: %w", err)
	}

	// Request PTY
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty(term, rows, cols, modes); err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH PTY: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stderr pipe: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("SSH stdin pipe: %w", err)
	}

	if err := session.Start(cmd); err != nil {
		session.Close()
		client.Close()
		return nil, nil, nil, nil, fmt.Errorf("docker exec: %w", err)
	}

	combined := io.MultiReader(stdout, stderr)
	return client, session, combined, stdin, nil
}

// DetectContainerShell probes for an available shell inside a container via SSH.
// Returns the first shell found, or tries a raw `sh` as last resort.
func DetectContainerShell(ctx context.Context, cfg Config, container string) string {
	candidates := []string{"/bin/bash", "/bin/sh", "/bin/ash", "/bin/dash", "bash", "sh", "ash"}
	for _, shell := range candidates {
		// Use sh -c to check if the shell binary exists inside the container
		checkCmd := fmt.Sprintf("docker exec %s sh -c 'command -v %s 2>/dev/null' 2>/dev/null || docker exec %s ls %s 2>/dev/null", container, shell, container, shell)
		out, err := RunCommand(ctx, cfg, checkCmd)
		if err == nil && strings.TrimSpace(out) != "" {
			return shell
		}
	}
	// Last resort — let docker resolve via PATH
	return "sh"
}

// ResizeTerminal sends a window change event to the SSH session
func ResizeTerminal(session *ssh.Session, rows, cols int) error {
	return session.WindowChange(rows, cols)
}

// ServerInfo holds auto-detected server information
type ServerInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	CPUCores int    `json:"cpu_cores"`
	CPUModel string `json:"cpu_model"`
}

// GetServerInfo auto-detects OS, kernel, CPU info from a remote server
func GetServerInfo(ctx context.Context, cfg Config) (*ServerInfo, error) {
	client, err := dial(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	info := &ServerInfo{}

	// Hostname
	if out, err := runCommand(client, "hostname"); err == nil {
		info.Hostname = strings.TrimSpace(string(out))
	}

	// OS + Kernel + Arch from uname
	if out, err := runCommand(client, "uname -s -r -m 2>/dev/null"); err == nil {
		parts := strings.Fields(string(out))
		if len(parts) >= 1 {
			info.Kernel = parts[0]
		}
		if len(parts) >= 2 {
			info.Kernel += " " + parts[1]
		}
		if len(parts) >= 3 {
			info.Arch = parts[2]
		}
	}

	// OS pretty name
	if out, err := runCommand(client, "cat /etc/os-release 2>/dev/null | grep -E '^PRETTY_NAME=' | cut -d= -f2 | tr -d '\"'"); err == nil {
		info.OS = strings.TrimSpace(string(out))
	}

	// CPU cores
	if out, err := runCommand(client, "nproc 2>/dev/null"); err == nil {
		fmt.Sscanf(string(out), "%d", &info.CPUCores)
	}

	// CPU model
	if out, err := runCommand(client, `cat /proc/cpuinfo 2>/dev/null | grep -m1 "model name" | cut -d: -f2 | sed 's/^ *//'`); err == nil {
		info.CPUModel = strings.TrimSpace(string(out))
	} else if out, err := runCommand(client, `lscpu 2>/dev/null | grep "Model name" | cut -d: -f2 | sed 's/^ *//'`); err == nil {
		info.CPUModel = strings.TrimSpace(string(out))
	}

	return info, nil
}

// GetNetTraffic fetches total received/transmitted bytes from /proc/net/dev
func GetNetTraffic(ctx context.Context, cfg Config) (rx, tx int64, err error) {
	client, dialErr := dial(ctx, cfg)
	if dialErr != nil {
		return 0, 0, dialErr
	}
	defer client.Close()

	out, cmdErr := runCommand(client,
		`cat /proc/net/dev | awk 'NR>2 {rx+=$2; tx+=$10} END {print rx, tx}'`,
	)
	if cmdErr != nil {
		return 0, 0, cmdErr
	}

	_, scanErr := fmt.Sscanf(string(out), "%d %d", &rx, &tx)
	return rx, tx, scanErr
}
