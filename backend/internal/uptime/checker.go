package uptime

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// ─── CheckResult ──────────────────────────────────────────────────────────

// CheckResult holds the outcome of a single uptime check.
type CheckResult struct {
	Status         string `json:"status"`          // "up", "down", "error"
	StatusCode     *int   `json:"status_code"`
	ResponseTimeMs *int   `json:"response_time_ms"`
	ErrorMessage   string `json:"error_message"`
}

// ─── HTTP Check ───────────────────────────────────────────────────────────

// CheckHTTP performs an HTTP(S) GET health check against the given URL.
// It measures response time, validates status code range, and optionally
// matches the response body against expectedBody (regex).
func CheckHTTP(url string, timeoutSec, expectedMin, expectedMax int, expectedBody string) *CheckResult {
	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	start := time.Now()
	resp, err := client.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		ms := calculateResponseMs(elapsed)
		errMsg := err.Error()
		// Make error messages user-friendly
		if strings.Contains(errMsg, "no such host") {
			errMsg = "DNS resolution failed"
		} else if strings.Contains(errMsg, "connection refused") {
			errMsg = "Connection refused"
		} else if strings.Contains(errMsg, "i/o timeout") {
			errMsg = "Request timed out"
		} else if strings.Contains(errMsg, "tls") {
			errMsg = "TLS handshake failed"
		} else if strings.Contains(errMsg, "EOF") {
			errMsg = "Connection closed unexpectedly"
		}
		return &CheckResult{
			Status:         "down",
			ResponseTimeMs: ms,
			ErrorMessage:   errMsg,
		}
	}
	defer resp.Body.Close()

	ms := calculateResponseMs(elapsed)

	// Validate status code range
	if resp.StatusCode < expectedMin || resp.StatusCode > expectedMax {
		msg := fmt.Sprintf("Unexpected status: %d (expected %d-%d)", resp.StatusCode, expectedMin, expectedMax)
		return &CheckResult{
			Status:         "down",
			StatusCode:     &resp.StatusCode,
			ResponseTimeMs: ms,
			ErrorMessage:   msg,
		}
	}

	// Validate expected body (regex match)
	if expectedBody != "" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		matched, err := regexp.MatchString(expectedBody, string(body))
		if err != nil {
			return &CheckResult{
				Status:         "error",
				StatusCode:     &resp.StatusCode,
				ResponseTimeMs: ms,
				ErrorMessage:   fmt.Sprintf("Body regex error: %s", err.Error()),
			}
		}
		if !matched {
			return &CheckResult{
				Status:         "down",
				StatusCode:     &resp.StatusCode,
				ResponseTimeMs: ms,
				ErrorMessage:   fmt.Sprintf("Body mismatch: expected pattern '%s' not found", expectedBody),
			}
		}
	}

	return &CheckResult{
		Status:         "up",
		StatusCode:     &resp.StatusCode,
		ResponseTimeMs: ms,
	}
}

// ─── TCP Check ────────────────────────────────────────────────────────────

// CheckTCP performs a TCP port check by dialing host:port.
func CheckTCP(host string, port int, timeoutSec int) *CheckResult {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	timeout := time.Duration(timeoutSec) * time.Second

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start)

	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "no such host") {
			errMsg = "DNS resolution failed"
		} else if strings.Contains(errMsg, "connection refused") {
			errMsg = "Connection refused"
		} else if strings.Contains(errMsg, "i/o timeout") {
			errMsg = "Connection timed out"
		}
		return &CheckResult{
			Status:         "down",
			ResponseTimeMs: calculateResponseMs(elapsed),
			ErrorMessage:   errMsg,
		}
	}
	conn.Close()

	return &CheckResult{
		Status:         "up",
		StatusCode:     intPtr(0),
		ResponseTimeMs: calculateResponseMs(elapsed),
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func calculateResponseMs(d time.Duration) *int {
	ms := int(d.Milliseconds())
	return &ms
}

func intPtr(i int) *int {
	return &i
}
