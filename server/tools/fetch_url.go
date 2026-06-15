package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type fetchArgs struct {
	URL string `json:"url"`
}

const (
	fetchTimeout  = 15 * time.Second
	fetchMaxBytes = 512 * 1024 // cap the body we read into the model's context (512 KiB)
)

// FetchURL GETs a URL and returns its body as text. It routes through the safehttp
// client so the model can never use it to reach internal infrastructure (Wall A).
// Tools never see credentials; this one needs none.
func FetchURL() Tool {
	safeClient := NewSafeClient(fetchTimeout)
	return Tool{
		Name:        "fetch_url",
		Description: "Fetch the contents of a public http(s) URL and return the response body as text.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"url": { "type": "string", "description": "the absolute http(s) URL to fetch" }
			},
			"required": ["url"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a fetchArgs
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			url := strings.TrimSpace(a.URL)
			if url == "" {
				return "", fmt.Errorf("url is empty")
			}
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				return "", fmt.Errorf("url must be http(s): %q", url)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return "", fmt.Errorf("bad request: %w", err)
			}
			req.Header.Set("User-Agent", "agent-builder/0.1 (+safehttp)")

			client := clientFrom(ctx, safeClient)
			resp, err := client.Do(req)
			if err != nil {
				return "", fmt.Errorf("fetch failed: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, fetchMaxBytes))
			if err != nil {
				return "", fmt.Errorf("reading body: %w", err)
			}
			text := string(body)
			if resp.StatusCode >= 400 {
				return "", fmt.Errorf("http %d: %s", resp.StatusCode, truncate(text, 500))
			}
			return text, nil
		},
	}
}

// truncate cuts s to at most n runes (not bytes) so it never splits a multibyte
// rune into invalid UTF-8 inside the model-visible error message.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
