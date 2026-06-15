package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeJoinRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	bad := []string{
		"/etc/passwd",      // absolute
		"../secret",        // parent escape
		"a/../../secret",   // nested escape
		"../../etc/passwd", // deep escape
		"",                 // empty
		"..",               // bare parent
	}
	for _, p := range bad {
		if _, err := safeJoin(root, p); err == nil {
			t.Errorf("safeJoin(%q) should be rejected", p)
		}
	}
	good := []string{"notes.txt", "a/b/c.txt", "./x.txt"}
	for _, p := range good {
		full, err := safeJoin(root, p)
		if err != nil {
			t.Errorf("safeJoin(%q) should be allowed: %v", p, err)
			continue
		}
		if !strings.HasPrefix(full, root) {
			t.Errorf("safeJoin(%q) = %q escaped root %q", p, full, root)
		}
	}
}

func TestReadWriteFileRoundTrip(t *testing.T) {
	root := t.TempDir()
	wf := WriteFile(root)
	rf := ReadFile(root)
	ctx := context.Background()

	if _, err := wf.Execute(ctx, json.RawMessage(`{"path":"sub/notes.txt","content":"hello"}`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	// file really exists on disk within root
	if _, err := os.Stat(filepath.Join(root, "sub", "notes.txt")); err != nil {
		t.Fatalf("file not written: %v", err)
	}
	out, err := rf.Execute(ctx, json.RawMessage(`{"path":"sub/notes.txt"}`))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out != "hello" {
		t.Errorf("read back %q, want hello", out)
	}

	// traversal is rejected at the tool boundary too
	if _, err := rf.Execute(ctx, json.RawMessage(`{"path":"../escape"}`)); err == nil {
		t.Error("read_file should reject ../ traversal")
	}
	if _, err := wf.Execute(ctx, json.RawMessage(`{"path":"/tmp/evil","content":"x"}`)); err == nil {
		t.Error("write_file should reject absolute path")
	}
	if wf.Consequential != true {
		t.Error("write_file must be consequential")
	}
}
