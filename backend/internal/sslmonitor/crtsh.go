package sslmonitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CRTSHCertificate represents a certificate from crt.sh.
type CRTSHCertificate struct {
	ID          int    `json:"id"`
	IssuerCAID  int    `json:"issuer_ca_id"`
	IssuerName  string `json:"issuer_name"`
	CommonName  string `json:"common_name"`
	NameValue   string `json:"name_value"`
	NotBefore   string `json:"not_before"`
	NotAfter    string `json:"not_after"`
	SerialNumber string `json:"serial_number"`
}

// LookupCRTSh queries crt.sh for certificate transparency logs for a domain.
func LookupCRTSh(domain string) ([]CRTSHCertificate, error) {
	url := fmt.Sprintf("https://crt.sh/?q=%s&output=json&limit=20&dedup=Y", domain)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("crt.sh query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh returned status %d", resp.StatusCode)
	}

	var certs []CRTSHCertificate
	if err := json.NewDecoder(resp.Body).Decode(&certs); err != nil {
		return nil, fmt.Errorf("crt.sh parse failed: %w", err)
	}
	if certs == nil {
		certs = []CRTSHCertificate{}
	}
	return certs, nil
}
