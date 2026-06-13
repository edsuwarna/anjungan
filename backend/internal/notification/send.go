package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/edsuwarna/anjungan/internal/common/model"
)

// SendSSLToTarget sends an SSL-formatted notification to the target.
func SendSSLToTarget(target *model.NotificationTarget, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	// For telegram, hot-path via Bot API with chat_id
	if target.Platform == "telegram" {
		text, err := FormatTelegramSSLNotification(payload)
		if err != nil {
			return 0, "", fmt.Errorf("format message: %w", err)
		}

		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", target.BotToken)
		botPayload := map[string]interface{}{
			"chat_id":    target.ChatID,
			"text":       string(text),
			"parse_mode": "HTML",
		}
		bodyBytes, err = json.Marshal(botPayload)
		if err != nil {
			return 0, "", fmt.Errorf("marshal bot payload: %w", err)
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "anjungan-sslmonitor-webhook/1.0")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("send: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody), nil
	}

	switch target.Platform {
	case "discord":
		bodyBytes, err = FormatDiscordSSLNotification(payload)
	case "slack":
		bodyBytes, err = FormatSlackSSLNotification(payload)
	default:
		bodyBytes, err = json.Marshal(payload)
	}

	if err != nil {
		return 0, "", fmt.Errorf("format message: %w", err)
	}

	req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "anjungan-sslmonitor-webhook/1.0")

	if target.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", target.WebhookSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// SendToTarget sends a platform-formatted notification to the target URL.
func SendToTarget(target *model.NotificationTarget, payload map[string]interface{}) (int, string, error) {
	var bodyBytes []byte
	var err error

	// For telegram, hot-path via Bot API with chat_id
	if target.Platform == "telegram" {
		text, err := formatTelegramUptimeNotification(payload)
		if err != nil {
			return 0, "", fmt.Errorf("format message: %w", err)
		}

		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", target.BotToken)
		botPayload := map[string]interface{}{
			"chat_id":    target.ChatID,
			"text":       string(text),
			"parse_mode": "HTML",
		}
		bodyBytes, err = json.Marshal(botPayload)
		if err != nil {
			return 0, "", fmt.Errorf("marshal bot payload: %w", err)
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "anjungan-webhook/1.0")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("send: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody), nil
	}

	switch target.Platform {
	case "discord":
		bodyBytes, err = formatDiscordUptimeNotification(payload)
	case "slack":
		bodyBytes, err = formatSlackUptimeNotification(payload)
	default:
		bodyBytes, err = json.Marshal(payload)
	}

	if err != nil {
		return 0, "", fmt.Errorf("format message: %w", err)
	}

	req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "anjungan-webhook/1.0")

	if target.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", target.WebhookSecret)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// SendRawJSON sends raw JSON payload to the target URL without platform-specific formatting.
func SendRawJSON(target *model.NotificationTarget, payload map[string]interface{}) (int, string, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "anjungan-webhook/1.0")
	if target.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", target.WebhookSecret)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// formatDiscordUptimeNotification formats payload as a Discord embed (Uptime Kuma style).
func formatDiscordUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	status, _ := payload["status"].(string)
	prevStatus, _ := payload["previous_status"].(string)
	checkType, _ := payload["check_type"].(string)
	msg, _ := payload["message"].(string)
	errorVal, _ := payload["error"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	var color int
	var statusEmoji string
	switch status {
	case "down":
		color = 0xEF4444
		statusEmoji = "🔴"
	case "up":
		color = 0x10B981
		statusEmoji = "🟢"
	default:
		color = 0x94A3B8
		statusEmoji = "⚪"
	}

	// Build status transition text
	statusTransition := statusEmoji + " " + status
	if prevStatus != "" && prevStatus != status {
		statusTransition += " (was " + prevStatus + ")"
	}

	// Build fields
	fields := []map[string]interface{}{
		{"name": "Service Name", "value": monitorName, "inline": true},
		{"name": "Check Type", "value": checkType, "inline": true},
		{"name": "Status", "value": statusTransition, "inline": false},
		{"name": "Service URL", "value": monitorURL, "inline": false},
	}

	// Status code — always shown if available
	if sc, ok := payload["status_code"].(int); ok && sc > 0 {
		fields = append(fields, map[string]interface{}{
			"name": "Status Code", "value": fmt.Sprintf("%d", sc), "inline": true,
		})
	}

	// Response time — always shown if available
	if ping, ok := payload["response_time_ms"].(float64); ok && ping > 0 {
		fields = append(fields, map[string]interface{}{
			"name": "Response Time", "value": fmt.Sprintf("%.0f ms", ping), "inline": true,
		})
	}

	// Error — shown when down
	if status == "down" && errorVal != "" {
		fields = append(fields, map[string]interface{}{
			"name": "Error", "value": errorVal, "inline": false,
		})
	}

	fields = append(fields, map[string]interface{}{
		"name": "Time (Asia/Jakarta)", "value": timestampWIB, "inline": false,
	})

	embed := map[string]interface{}{
		"title":       msg,
		"color":       color,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"fields":      fields,
	}

	return json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	})
}

// formatTelegramUptimeNotification formats payload as a Telegram message (Uptime Kuma style).
func formatTelegramUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	status, _ := payload["status"].(string)
	prevStatus, _ := payload["previous_status"].(string)
	checkType, _ := payload["check_type"].(string)
	msg, _ := payload["message"].(string)
	errorVal, _ := payload["error"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	var statusEmoji string
	switch status {
	case "up":
		statusEmoji = "🟢"
	case "down":
		statusEmoji = "🔴"
	default:
		statusEmoji = "⚪"
	}

	// Build status transition
	statusLine := statusEmoji + " **" + status + "**"
	if prevStatus != "" && prevStatus != status {
		statusLine += " (was " + prevStatus + ")"
	}

	text := msg + "\n\n"
	text += fmt.Sprintf("**Service Name:** %s\n", monitorName)
	text += fmt.Sprintf("**Service URL:** %s\n", monitorURL)
	text += fmt.Sprintf("**Check Type:** %s\n", checkType)
	text += fmt.Sprintf("**Status:** %s\n", statusLine)

	if sc, ok := payload["status_code"].(int); ok && sc > 0 {
		text += fmt.Sprintf("**Status Code:** %d\n", sc)
	}

	if ping, ok := payload["response_time_ms"].(float64); ok && ping > 0 {
		text += fmt.Sprintf("**Response Time:** %.0f ms\n", ping)
	}

	if status == "down" && errorVal != "" {
		text += fmt.Sprintf("**Error:** %s\n", errorVal)
	}

	text += fmt.Sprintf("**Time (Asia/Jakarta):** %s\n", timestampWIB)

	return json.Marshal(map[string]interface{}{
		"text":                  text,
		"parse_mode":            "Markdown",
		"disable_web_page_preview": true,
	})
}

// formatSlackUptimeNotification formats payload as a Slack message (Uptime Kuma style).
func formatSlackUptimeNotification(payload map[string]interface{}) ([]byte, error) {
	monitorName, _ := payload["monitor_name"].(string)
	monitorURL, _ := payload["monitor_url"].(string)
	status, _ := payload["status"].(string)
	prevStatus, _ := payload["previous_status"].(string)
	checkType, _ := payload["check_type"].(string)
	msg, _ := payload["message"].(string)
	errorVal, _ := payload["error"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	var emoji string
	switch status {
	case "down":
		emoji = ":red_circle:"
	case "up":
		emoji = ":white_check_mark:"
	default:
		emoji = ":white_circle:"
	}

	// Build status transition
	statusLine := emoji + " *" + status + "*"
	if prevStatus != "" && prevStatus != status {
		statusLine += " (was " + prevStatus + ")"
	}

	// Build fields text
	fieldsText := fmt.Sprintf("*Service Name:* %s\n*Service URL:* %s\n*Check Type:* %s\n*Status:* %s",
		monitorName, monitorURL, checkType, statusLine)

	if sc, ok := payload["status_code"].(int); ok && sc > 0 {
		fieldsText += fmt.Sprintf("\n*Status Code:* %d", sc)
	}

	if ping, ok := payload["response_time_ms"].(float64); ok && ping > 0 {
		fieldsText += fmt.Sprintf("\n*Response Time:* %.0f ms", ping)
	}

	if status == "down" && errorVal != "" {
		fieldsText += fmt.Sprintf("\n*Error:* %s", errorVal)
	}

	fieldsText += fmt.Sprintf("\n*Time (Asia/Jakarta):* %s", timestampWIB)

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s %s", emoji, msg),
			},
		},
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fieldsText,
			},
		},
	}

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf("%s Uptime Alert: %s", emoji, monitorName),
		"blocks": blocks,
	})
}

// TruncateString truncates a string to the given max length.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ─── SSL Notification Formatters (Uptime Kuma style) ─────────────────────────

// FormatDiscordSSLNotification formats payload as a Discord embed (SSL-specific).
func FormatDiscordSSLNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	port, _ := payload["port"].(int)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	expiresAt, _ := payload["expires_at"].(string)
	cipherGrade, _ := payload["cipher_grade"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	var color int
	switch status {
	case "expired":
		color = 0xEF4444
	case "expiring_soon":
		color = 0xF59E0B
	case "valid":
		color = 0x10B981
	default:
		color = 0x94A3B8
	}

	fields := []map[string]interface{}{
		{"name": "Domain", "value": fmt.Sprintf("%s:%d", domain, port), "inline": true},
		{"name": "Days Remaining", "value": fmt.Sprintf("%d days", days), "inline": true},
		{"name": "Issuer", "value": issuer, "inline": false},
		{"name": "Cipher Grade", "value": cipherGrade, "inline": true},
		{"name": "Expires At", "value": expiresAt, "inline": false},
		{"name": "Time (Asia/Jakarta)", "value": timestampWIB, "inline": false},
	}

	if status == "expired" {
		if err, ok := payload["error"].(string); ok && err != "" {
			fields = append(fields, map[string]interface{}{
				"name": "Error", "value": err, "inline": false,
			})
		}
	}

	embed := map[string]interface{}{
		"title":     msg,
		"color":     color,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"fields":    fields,
	}

	return json.Marshal(map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	})
}

// FormatTelegramSSLNotification formats payload as a Telegram message (SSL-specific).
func FormatTelegramSSLNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	port, _ := payload["port"].(int)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	cipherGrade, _ := payload["cipher_grade"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	text := msg + "\n\n"
	text += fmt.Sprintf("Domain: %s:%d\n", domain, port)
	text += fmt.Sprintf("Days Remaining: %d\n", days)
	text += fmt.Sprintf("Issuer: %s\n", issuer)
	text += fmt.Sprintf("Cipher Grade: %s\n", cipherGrade)
	text += fmt.Sprintf("Time (Asia/Jakarta): %s\n", timestampWIB)

	if status == "expired" || status == "error" {
		if errVal, ok := payload["error"].(string); ok && errVal != "" {
			text += fmt.Sprintf("Error: %s\n", errVal)
		}
	}

	return json.Marshal(map[string]interface{}{
		"text":                     text,
		"parse_mode":               "Markdown",
		"disable_web_page_preview": true,
	})
}

// FormatSlackSSLNotification formats payload as a Slack message (SSL-specific).
func FormatSlackSSLNotification(payload map[string]interface{}) ([]byte, error) {
	domain, _ := payload["domain"].(string)
	port, _ := payload["port"].(int)
	status, _ := payload["status"].(string)
	msg, _ := payload["message"].(string)
	days, _ := payload["days_remaining"].(int)
	issuer, _ := payload["issuer"].(string)
	cipherGrade, _ := payload["cipher_grade"].(string)
	timestampWIB, _ := payload["timestamp_wib"].(string)

	var emoji string
	switch status {
	case "expired":
		emoji = ":red_circle:"
	case "expiring_soon":
		emoji = ":warning:"
	case "valid":
		emoji = ":white_check_mark:"
	default:
		emoji = ":white_circle:"
	}

	fieldsText := fmt.Sprintf("*Domain:* %s:%d\n*Days Remaining:* %d\n*Issuer:* %s\n*Cipher Grade:* %s\n*Time (Asia/Jakarta):* %s",
		domain, port, days, issuer, cipherGrade, timestampWIB)

	if status == "expired" || status == "error" {
		if errVal, ok := payload["error"].(string); ok && errVal != "" {
			fieldsText += fmt.Sprintf("\n*Error:* %s", errVal)
		}
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("%s %s", emoji, msg),
			},
		},
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fieldsText,
			},
		},
	}

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf("%s SSL Alert: %s", emoji, domain),
		"blocks": blocks,
	})
}

// ─── Security Alert (Brute Force) ──────────────────────────────────────────────

// SendBruteForceAlert sends a brute force alert notification to the target.
func SendBruteForceAlert(target *model.NotificationTarget, ipAddress string, failures, windowMinutes, userCount int, firstAttempt, lastAttempt string) (int, string, error) {
	// Telegram: hot-path via Bot API
	if target.Platform == "telegram" {
		text := formatTelegramBruteForceAlert(ipAddress, failures, windowMinutes, userCount, firstAttempt, lastAttempt)
		apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", target.BotToken)
		botPayload := map[string]interface{}{
			"chat_id":    target.ChatID,
			"text":       text,
			"parse_mode": "HTML",
		}
		bodyBytes, _ := json.Marshal(botPayload)
		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("send: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody), nil
	}

	// Discord
	if target.Platform == "discord" {
		bodyBytes, err := formatDiscordBruteForceAlert(ipAddress, failures, windowMinutes, userCount, firstAttempt, lastAttempt)
		if err != nil {
			return 0, "", err
		}
		req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("send: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody), nil
	}

	// Slack
	if target.Platform == "slack" {
		bodyBytes, err := formatSlackBruteForceAlert(ipAddress, failures, windowMinutes, userCount, firstAttempt, lastAttempt)
		if err != nil {
			return 0, "", err
		}
		req, err := http.NewRequest("POST", target.URL, bytes.NewReader(bodyBytes))
		if err != nil {
			return 0, "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return 0, "", fmt.Errorf("send: %w", err)
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(respBody), nil
	}

	// Generic: send raw JSON
	payload := map[string]interface{}{
		"type":           "brute_force",
		"ip_address":     ipAddress,
		"failures":       failures,
		"window_minutes": windowMinutes,
		"user_count":     userCount,
		"first_attempt":  firstAttempt,
		"last_attempt":   lastAttempt,
	}
	return SendRawJSON(target, payload)
}

func formatTelegramBruteForceAlert(ipAddress string, failures, windowMinutes, userCount int, firstAttempt, lastAttempt string) string {
	eventType := "Brute Force"
	if userCount > 5 {
		eventType = "Credential Stuffing"
	}

	text := fmt.Sprintf(`🚨 <b>%s Attack Detected</b>

<b>IP:</b> <code>%s</code>
<b>Failures:</b> %d in %d minutes
<b>Users affected:</b> %d
<b>First attempt:</b> %s
<b>Last attempt:</b> %s`,
		eventType, ipAddress, failures, windowMinutes, userCount, firstAttempt, lastAttempt)

	return text
}

func formatDiscordBruteForceAlert(ipAddress string, failures, windowMinutes, userCount int, firstAttempt, lastAttempt string) ([]byte, error) {
	eventType := "Brute Force"
	if userCount > 5 {
		eventType = "Credential Stuffing"
	}

	color := 0xEF4444 // red
	embed := map[string]interface{}{
		"title": fmt.Sprintf("🚨 %s Attack Detected", eventType),
		"color": color,
		"fields": []map[string]interface{}{
			{"name": "IP Address", "value": ipAddress, "inline": true},
			{"name": "Failures", "value": fmt.Sprintf("%d in %d min", failures, windowMinutes), "inline": true},
			{"name": "Users Affected", "value": fmt.Sprintf("%d", userCount), "inline": true},
			{"name": "Period", "value": fmt.Sprintf("%s — %s", firstAttempt, lastAttempt), "inline": false},
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	return json.Marshal(map[string]interface{}{
		"content": fmt.Sprintf("🚨 **%s** attack detected from `%s`", eventType, ipAddress),
		"embeds":  []map[string]interface{}{embed},
	})
}

func formatSlackBruteForceAlert(ipAddress string, failures, windowMinutes, userCount int, firstAttempt, lastAttempt string) ([]byte, error) {
	eventType := "Brute Force"
	if userCount > 5 {
		eventType = "Credential Stuffing"
	}

	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf(":red_circle: *%s Attack Detected*", eventType),
			},
		},
		{
			"type": "section",
			"fields": []map[string]interface{}{
				{"type": "mrkdwn", "text": fmt.Sprintf("*IP:*\n`%s`", ipAddress)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Failures:*\n%d in %d min", failures, windowMinutes)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Users Affected:*\n%d", userCount)},
			},
		},
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Period:* %s — %s", firstAttempt, lastAttempt),
			},
		},
	}

	return json.Marshal(map[string]interface{}{
		"text":   fmt.Sprintf(":red_circle: %s Attack Detected from %s", eventType, ipAddress),
		"blocks": blocks,
	})
}
