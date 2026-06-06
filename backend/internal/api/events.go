package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EventHub is a fan-out SSE broadcaster.
// Call Broadcast() from any handler after a mutation.
// Register it on GET /api/v1/events (inside the auth group).
type EventHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{clients: make(map[chan []byte]struct{})}
}

func (h *EventHub) register() (chan []byte, func()) {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		// drain so the goroutine can exit
		for len(ch) > 0 {
			<-ch
		}
	}
}

// Broadcast sends a change event to all connected SSE clients.
// eventType examples: "task:change", "objective:change", "plan:change", "tag:change"
// meta can carry contextual fields like date or week_start.
func (h *EventHub) Broadcast(eventType string, meta map[string]string) {
	payload := map[string]string{"type": eventType}
	for k, v := range meta {
		payload[k] = v
	}
	data, _ := json.Marshal(payload)
	msg := fmt.Sprintf("event: change\ndata: %s\n\n", data)
	raw := []byte(msg)

	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- raw:
		default: // slow client — skip rather than block
		}
	}
}

// ServeSSE handles GET /api/v1/events — server-sent events stream.
// This handler is registered INSIDE the auth group so requireAuth runs first.
func (h *EventHub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher.Flush()

	ch, cleanup := h.register()
	defer cleanup()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
