package ratelimit

import "testing"

func TestAllowBurstThenBlock(t *testing.T) {
	l := New()
	if !l.Allow("k", "2/s") {
		t.Fatal("first")
	}
	if !l.Allow("k", "2/s") {
		t.Fatal("second")
	}
	if l.Allow("k", "2/s") {
		t.Fatal("expected block")
	}
}
