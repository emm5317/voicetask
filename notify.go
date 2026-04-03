package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// ntfyPriority maps task priority to ntfy priority levels (1-5).
var ntfyPriority = map[string]string{
	"urgent": "5",
	"high":   "4",
	"normal": "3",
	"low":    "2",
}

// notifyTaskCreated sends a push notification via Ntfy for a new task.
// Runs in a goroutine — never blocks the request.
func (a *App) notifyTaskCreated(title, tag, priority string) {
	if a.cfg.NtfyURL == "" || a.cfg.NtfyTopic == "" {
		return
	}

	go func() {
		url := fmt.Sprintf("%s/%s", strings.TrimRight(a.cfg.NtfyURL, "/"), a.cfg.NtfyTopic)
		body := fmt.Sprintf("%s — %s (%s)", title, tag, priority)

		req, err := http.NewRequest("POST", url, strings.NewReader(body))
		if err != nil {
			slog.Error("ntfy: create request", "err", err)
			return
		}
		req.Header.Set("Title", "VoiceTask: "+title)
		req.Header.Set("Tags", tag)
		if p, ok := ntfyPriority[priority]; ok {
			req.Header.Set("Priority", p)
		}

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("ntfy: send", "err", err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode >= 300 {
			slog.Warn("ntfy: unexpected status", "status", resp.StatusCode)
		}
	}()
}
