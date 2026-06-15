package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// sseWriter streams Server-Sent Events. Each send writes one `data: <json>\n\n` frame
// and flushes, so the client sees events live (00-CONTEXT §7). Writes happen only from
// the single request goroutine (the engine emits synchronously), so no lock is needed.
type sseWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

// newSSE writes the SSE response headers and returns a writer, or an error if the
// ResponseWriter cannot flush (no streaming possible). Call before emitting any event.
func newSSE(w http.ResponseWriter) (*sseWriter, error) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming unsupported")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // disable proxy buffering
	w.WriteHeader(http.StatusOK)
	f.Flush()
	return &sseWriter{w: w, f: f}, nil
}

// send marshals v and writes it as one SSE frame. A write error means the client is
// gone; the caller relies on request-context cancellation to stop the run.
func (s *sseWriter) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", b); err != nil {
		return err
	}
	s.f.Flush()
	return nil
}
