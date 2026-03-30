package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// Alert is the JSON payload sent to the webhook URL.
type Alert struct {
	Event     string         `json:"event"`     // "risk_alert"
	UUID      string         `json:"uuid"`
	Type      string         `json:"type"`
	Detail    map[string]any `json:"detail"`
	Timestamp string         `json:"timestamp"` // RFC3339
}

// SendAsync posts the alert to url in a background goroutine.
// Failures are silently ignored.
func SendAsync(url string, alert Alert) {
	if url == "" {
		return
	}
	alert.Timestamp = time.Now().UTC().Format(time.RFC3339)
	go send(url, alert)
}

func send(url string, alert Alert) {
	body, err := json.Marshal(alert)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}
