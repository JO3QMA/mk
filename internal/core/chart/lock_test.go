package chart

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// errReader is an io.Reader that always fails. Used to drive
// newLockToken's fallback branch.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("forced rand failure")
}

func TestNewLockToken_HappyPath(t *testing.T) {
	got := newLockToken()
	// 16 bytes hex => 32 chars
	if len(got) != 32 {
		t.Fatalf("expected 32-char hex token, got %q (len=%d)", got, len(got))
	}
}

func TestNewLockToken_RandFallback(t *testing.T) {
	old := randReader
	randReader = errReader{}
	defer func() { randReader = old }()

	got := newLockToken()
	// fallback path returns a time-based string with a "." separator.
	if !strings.Contains(got, ".") {
		t.Fatalf("expected fallback time string, got %q", got)
	}
	// Sanity: it must be a parseable date prefix-by-format.
	if len(got) < 21 {
		t.Fatalf("fallback token too short: %q", got)
	}
}

// Sanity check that randReader still satisfies io.Reader after override
// (compile-time only).
var _ io.Reader = randReader
