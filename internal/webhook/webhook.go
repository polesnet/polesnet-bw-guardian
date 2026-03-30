package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Alert is the JSON payload sent to the webhook URL.
type Alert struct {
	Event     string         `json:"event"` // "risk_alert"
	UUID      string         `json:"uuid"`
	Type      string         `json:"type"`
	Detail    map[string]any `json:"detail"`
	Timestamp string         `json:"timestamp"` // RFC3339
}

var (
	wg         sync.WaitGroup
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

// SendAsync posts the alert to url in a background goroutine.
// Failures are silently ignored.
func SendAsync(url string, alert Alert) {
	if url == "" {
		return
	}
	alert.Timestamp = time.Now().UTC().Format(time.RFC3339)
	wg.Add(1)
	go func() {
		defer wg.Done()
		send(url, alert)
	}()
}

// Wait blocks until all pending webhook requests have completed.
// Call this before process exit (e.g., at the end of cmdRun).
func Wait() {
	wg.Wait()
}

func send(url string, alert Alert) {
	body, err := json.Marshal(alert)
	if err != nil {
		return
	}
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	resp.Body.Close()
}
