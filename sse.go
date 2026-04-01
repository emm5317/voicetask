package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

// SSEHub manages Server-Sent Event client connections and broadcasts.
type SSEHub struct {
	clients map[chan string]bool
	mu      sync.RWMutex
}

// NewSSEHub creates a new SSE hub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[chan string]bool),
	}
}

// Subscribe adds a new client channel and returns it.
func (h *SSEHub) Subscribe() chan string {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan string, 16)
	h.clients[ch] = true
	slog.Info("sse client connected", "total", len(h.clients))
	return ch
}

// Unsubscribe removes and closes a client channel.
func (h *SSEHub) Unsubscribe(ch chan string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[ch]; ok {
		delete(h.clients, ch)
		close(ch)
		slog.Info("sse client disconnected", "total", len(h.clients))
	}
}

// Broadcast sends an event to all connected clients.
// Non-blocking: skips clients with full buffers.
func (h *SSEHub) Broadcast(event, data string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg := fmt.Sprintf("event: %s\ndata: %s\n\n", event, data)
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			slog.Warn("sse: skipped slow client")
		}
	}
}

// HandleStream is the Fiber handler for GET /tasks/stream.
// It holds the connection open and streams SSE events to the client.
func (h *SSEHub) HandleStream(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		ch := h.Subscribe()
		defer h.Unsubscribe(ch)

		// Set reconnection interval and send initial keepalive
		fmt.Fprintf(w, "retry: 5000\n\n")
		fmt.Fprintf(w, ": keepalive\n\n")
		w.Flush()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprint(w, msg)
				if err := w.Flush(); err != nil {
					return // client disconnected
				}
			case <-ticker.C:
				// Send keepalive comment to detect dead connections
				fmt.Fprintf(w, ": keepalive\n\n")
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})

	return nil
}

// StreamWriterFunc is the type expected by fasthttp's SetBodyStreamWriter.
type StreamWriterFunc = func(w *bufio.Writer)

// Ensure fasthttp compatibility at compile time.
var _ fasthttp.StreamWriter = StreamWriterFunc(nil)
