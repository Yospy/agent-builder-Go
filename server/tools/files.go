package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileMaxBytes = 256 * 1024 // cap read/write size

// ReadFile returns a tool that reads a file confined to root. Path traversal (absolute
// paths or "..") is rejected so the model can never read outside the working dir.
func ReadFile(root string) Tool {
	return Tool{
		Name:        "read_file",
		Description: "Read a UTF-8 text file from the working directory by relative path.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": { "path": { "type": "string", "description": "relative path within the working directory" } },
			"required": ["path"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			full, err := safeJoin(root, a.Path)
			if err != nil {
				return "", err
			}
			info, err := os.Stat(full)
			if err != nil {
				return "", fmt.Errorf("cannot read %q: %w", a.Path, err)
			}
			if info.IsDir() {
				return "", fmt.Errorf("%q is a directory", a.Path)
			}
			if info.Size() > fileMaxBytes {
				return "", fmt.Errorf("%q is too large (max %d bytes)", a.Path, fileMaxBytes)
			}
			b, err := os.ReadFile(full)
			if err != nil {
				return "", fmt.Errorf("cannot read %q: %w", a.Path, err)
			}
			return string(b), nil
		},
	}
}

// WriteFile returns a tool that writes a file confined to root. It is consequential.
func WriteFile(root string) Tool {
	return Tool{
		Name:          "write_file",
		Description:   "Write a UTF-8 text file in the working directory by relative path (creates parent dirs).",
		Consequential: true,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path":    { "type": "string", "description": "relative path within the working directory" },
				"content": { "type": "string", "description": "the file contents" }
			},
			"required": ["path", "content"],
			"additionalProperties": false
		}`),
		Execute: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var a struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}
			if len(a.Content) > fileMaxBytes {
				return "", fmt.Errorf("content too large (max %d bytes)", fileMaxBytes)
			}
			full, err := safeJoin(root, a.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", fmt.Errorf("cannot create dir for %q: %w", a.Path, err)
			}
			if err := os.WriteFile(full, []byte(a.Content), 0o644); err != nil {
				return "", fmt.Errorf("cannot write %q: %w", a.Path, err)
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
		},
	}
}

// safeJoin resolves a relative path against root and guarantees the result stays
// inside root — rejecting absolute paths and "../" escapes (the path-traversal edge
// case in 00-CONTEXT §11). This is a LEXICAL check: it assumes the working dir does
// not contain symlinks pointing outside root (true for the v1 fresh workspace/). If
// untrusted symlinks become possible, add an EvalSymlinks containment re-check here.
func safeJoin(root, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("path is empty")
	}
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("path must be relative, not absolute: %q", p)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("bad root: %w", err)
	}
	full := filepath.Join(absRoot, p)
	rel, err := filepath.Rel(absRoot, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the working directory: %q", p)
	}
	return full, nil
}
