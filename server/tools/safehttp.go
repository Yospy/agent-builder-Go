package tools

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// safehttp is Wall A (SSRF). Every LLM-influenced outbound fetch goes through this
// client, which refuses to open a socket to an internal address. The check runs in
// net.Dialer.Control — i.e. on the ACTUAL resolved IP about to be dialed, after DNS
// and again on every redirect hop. That is what defeats the two string-check evasions:
//   - redirect: public URL 302s to http://169.254.169.254/... (re-checked per hop)
//   - DNS rebinding (TOCTOU): name resolves public at check time, internal at dial time
// A URL-string check sees neither; the dial-time IP check sees both.

// cgnat is the carrier-grade NAT range 100.64.0.0/10 (RFC 6598) — not covered by
// IsPrivate but routable to internal infrastructure, so we deny it too.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// blockedIP reports whether an IP must never be dialed by an LLM-influenced request.
// Fails closed: anything not provably a public unicast address is rejected. Covers
// loopback, private (RFC1918 + IPv6 ULA via IsPrivate), link-local (incl. the
// 169.254.169.254 cloud metadata endpoint), multicast, CGNAT, and unspecified.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true // unparseable -> refuse, fail closed
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		cgnatContains(ip)
}

// cgnatContains reports whether ip is in the CGNAT range 100.64.0.0/10.
func cgnatContains(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && cgnat.Contains(v4)
}

// controlBlockInternal is the Dialer.Control hook. address is "host:port" with host
// already resolved to a literal IP by the dialer, so we validate the real target.
func controlBlockInternal(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("safehttp: bad dial address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if blockedIP(ip) {
		return fmt.Errorf("safehttp: blocked dial to internal address %s", host)
	}
	return nil
}

// NewSafeClient returns an *http.Client whose every connection (initial + each
// redirect) is gated by controlBlockInternal. timeout bounds the whole request.
func NewSafeClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   controlBlockInternal,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("safehttp: stopped after 10 redirects")
			}
			return nil // each redirect still dials through Control above
		},
	}
}

// ctxKeyClient lets tests inject a stub client; nil falls back to the real safe one.
type ctxKeyClient struct{}

func clientFrom(ctx context.Context, fallback *http.Client) *http.Client {
	if c, ok := ctx.Value(ctxKeyClient{}).(*http.Client); ok && c != nil {
		return c
	}
	return fallback
}

// WithHTTPClient returns a context that makes fetch_url use the given client. Used by
// tests to exercise fetch logic without real network egress.
func WithHTTPClient(ctx context.Context, c *http.Client) context.Context {
	return context.WithValue(ctx, ctxKeyClient{}, c)
}
