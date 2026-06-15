package runtime

import "sync"

// confirms tracks consequential tool calls that are paused awaiting human approval.
// Keyed by "<sessionID>:<callID>". The engine's Confirm hook registers a channel
// BEFORE the confirm event is emitted (the two-phase hook in engine.Input), then
// blocks; POST /approve delivers the decision (00-CONTEXT §8).
//
// Because registration strictly precedes the event reaching the client, deliver
// finding no channel genuinely means "unknown call_id" (-> 404) — never a race.
type confirms struct {
	mu sync.Mutex
	m  map[string]chan bool
}

func newConfirms() *confirms { return &confirms{m: make(map[string]chan bool)} }

// register creates a buffered (cap 1) channel for key and returns it. The buffer means
// deliver never blocks even if the waiter is between select iterations.
func (c *confirms) register(key string) chan bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan bool, 1)
	c.m[key] = ch
	return ch
}

// deliver sends a decision to a pending confirmation. Returns false if no confirmation
// is pending for key (caller responds 404).
func (c *confirms) deliver(key string, approved bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch, ok := c.m[key]
	if !ok {
		return false
	}
	select {
	case ch <- approved:
	default: // already delivered; ignore the duplicate
	}
	return true
}

func (c *confirms) drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

// inflight enforces one active run per session (00-CONTEXT §11: double-submit -> 409).
type inflight struct {
	mu sync.Mutex
	m  map[string]bool
}

func newInflight() *inflight { return &inflight{m: make(map[string]bool)} }

// acquire returns false if a run is already in flight for id.
func (i *inflight) acquire(id string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.m[id] {
		return false
	}
	i.m[id] = true
	return true
}

func (i *inflight) release(id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.m, id)
}
