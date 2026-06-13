package authactivity

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ipAPIResponse represents the response from ip-api.com
type ipAPIResponse struct {
	Status      string `json:"status"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	AS          string `json:"as"`
	ISP         string `json:"isp"`
}

// lookupGeo fetches country code, ASN, and ISP for an IP address.
// Uses ip-api.com free tier (45 req/min limit, no key needed).
// Best-effort — returns empty strings on failure.
func lookupGeo(ip string) (country, asn, isp string) {
	ip = CleanIP(ip)
	if ip == "" || isPrivateIP(ip) {
		return "", "", ""
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode,as,isp", ip))
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()

	var result ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", ""
	}
	if result.Status != "success" {
		return "", "", ""
	}

	return result.CountryCode, result.AS, result.ISP
}

// isPrivateIP returns true if ip is a private/reserved address.
func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return true
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() || parsed.IsUnspecified() {
		return true
	}
	// Also check common Docker/VPN ranges
	for _, cidr := range []string{
		"172.16.0.0/12",
		"10.0.0.0/8",
		"192.168.0.0/16",
		"100.64.0.0/10",
		"198.18.0.0/15",
	} {
		_, cidrNet, _ := net.ParseCIDR(cidr)
		if cidrNet != nil && cidrNet.Contains(parsed) {
			return true
		}
	}
	return false
}

// parseASN extracts just the AS number from the full "AS<num> <org>" string.
// Returns the full string if it's already just an AS number or empty.
func parseASN(as string) string {
	if as == "" {
		return ""
	}
	parts := strings.SplitN(as, " ", 2)
	return parts[0]
}
