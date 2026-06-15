package tools

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"169.254.169.254", true}, // cloud metadata endpoint — the crown jewel
		{"127.0.0.1", true},       // loopback
		{"::1", true},             // IPv6 loopback
		{"10.0.0.5", true},        // private
		{"172.16.3.4", true},      // private
		{"192.168.1.1", true},     // private
		{"0.0.0.0", true},         // unspecified
		{"fe80::1", true},         // IPv6 link-local
		{"fc00::1", true},         // IPv6 unique-local (private)
		{"100.64.0.1", true},      // carrier-grade NAT (RFC 6598)
		{"224.0.0.1", true},       // multicast
		{"8.8.8.8", false},        // public
		{"1.1.1.1", false},        // public
		{"93.184.216.34", false},  // public (example.com)
	}
	for _, c := range cases {
		got := blockedIP(net.ParseIP(c.ip))
		if got != c.blocked {
			t.Errorf("blockedIP(%s) = %v, want %v", c.ip, got, c.blocked)
		}
	}
	if !blockedIP(nil) {
		t.Error("blockedIP(nil) must be true (fail closed)")
	}
}

func TestControlBlocksInternalAddress(t *testing.T) {
	// The Control hook must reject an internal dial target and allow a public one.
	if err := controlBlockInternal("tcp", "169.254.169.254:80", nil); err == nil {
		t.Error("controlBlockInternal allowed the metadata endpoint")
	}
	if err := controlBlockInternal("tcp", "8.8.8.8:443", nil); err != nil {
		t.Errorf("controlBlockInternal blocked a public address: %v", err)
	}
}

// Integration: the chokepoint must be live in the actual *http.Client, not just in
// the standalone predicate. A loopback httptest server is itself an internal target,
// so the safe client must refuse to dial it — proving Dialer.Control is wired in.
func TestSafeClientBlocksLoopbackServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("internal-secret"))
	}))
	defer srv.Close()

	client := NewSafeClient(5 * time.Second)
	resp, err := client.Get(srv.URL) // srv.URL is http://127.0.0.1:PORT
	if err == nil {
		resp.Body.Close()
		t.Fatal("safe client must refuse to dial a loopback server")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error = %v, want it to mention 'blocked'", err)
	}
}

func TestCalculator(t *testing.T) {
	calc := Calculator()
	ok := []struct{ expr, want string }{
		{`{"expression":"2 + 2"}`, "4"},
		{`{"expression":"2 + 3 * (4 - 1)"}`, "11"},
		{`{"expression":"10 / 4"}`, "2.5"}, // real division, not integer truncation
		{`{"expression":"-3 + 5"}`, "2"},   // unary minus
	}
	for _, c := range ok {
		got, err := calc.Execute(context.Background(), json.RawMessage(c.expr))
		if err != nil {
			t.Errorf("calc(%s) errored: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("calc(%s) = %q, want %q", c.expr, got, c.want)
		}
	}

	bad := []string{
		`{"expression":""}`,           // empty
		`{"expression":"1/0"}`,        // div by zero
		`{"expression":"os.Exit(1)"}`, // not a constant expression -> rejected
		`not json`,                    // malformed args
	}
	for _, b := range bad {
		if _, err := calc.Execute(context.Background(), json.RawMessage(b)); err == nil {
			t.Errorf("calc(%s) should have errored", b)
		}
	}
}

func TestFetchURLValidatesScheme(t *testing.T) {
	fetch := FetchURL()
	for _, bad := range []string{`{"url":""}`, `{"url":"ftp://x"}`, `{"url":"file:///etc/passwd"}`, `bad json`} {
		if _, err := fetch.Execute(context.Background(), json.RawMessage(bad)); err == nil {
			t.Errorf("fetch_url(%s) should have errored", bad)
		}
	}
}

func TestFetchURLWithInjectedClient(t *testing.T) {
	// Exercise the fetch path without real egress by injecting a stub client.
	stub := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("hello world")),
			Header:     make(http.Header),
		}, nil
	})}
	ctx := WithHTTPClient(context.Background(), stub)
	out, err := FetchURL().Execute(ctx, json.RawMessage(`{"url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("fetch errored: %v", err)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("got %q, want it to contain 'hello world'", out)
	}

	// A 4xx must surface as an error.
	stub.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Body: io.NopCloser(strings.NewReader("nope")), Header: make(http.Header)}, nil
	})
	if _, err := FetchURL().Execute(ctx, json.RawMessage(`{"url":"https://example.com"}`)); err == nil {
		t.Error("fetch_url on a 404 should error")
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(Calculator())
	if !r.Has("calculator") {
		t.Error("registry should have calculator")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("registry should not have 'nope'")
	}
	if len(r.Names()) != 1 {
		t.Errorf("Names() = %v, want 1 entry", r.Names())
	}

	// duplicate registration must panic (wiring bug, fail fast)
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	r.Register(Calculator())
}

// --- tiny test helpers ---

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
