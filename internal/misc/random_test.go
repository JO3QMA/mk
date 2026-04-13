package misc

import (
	"testing"
)

func TestSecureRandomHex_Length(t *testing.T) {
	for _, n := range []int{8, 16, 32, 64} {
		s := SecureRandomHex(n)
		if len(s) != n {
			t.Errorf("SecureRandomHex(%d) returned length %d", n, len(s))
		}
	}
}

func TestSecureRandomHex_Unique(t *testing.T) {
	a := SecureRandomHex(32)
	b := SecureRandomHex(32)
	if a == b {
		t.Error("two calls returned the same value")
	}
}

func TestSecureRandomHex_OddLength(t *testing.T) {
	s := SecureRandomHex(7)
	if len(s) != 7 {
		t.Errorf("expected length 7, got %d", len(s))
	}
}
