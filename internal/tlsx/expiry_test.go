package tlsx

import (
	"testing"
	"time"
)

func TestExpiryAlertLevel(t *testing.T) {
	tests := []struct {
		days, warn int
		want       int
	}{
		{20, 14, -1},
		{14, 14, 14},
		{13, 14, 14},
		{8, 14, 14},
		{7, 14, 7},
		{4, 14, 7},
		{1, 14, 1},
		{0, 14, 1},
		{-3, 14, 0},
		{6, 7, 7},
		{10, 7, -1},
		{20, 30, 30},
	}
	for _, tc := range tests {
		got := expiryAlertLevel(tc.days, tc.warn)
		if got != tc.want {
			t.Fatalf("expiryAlertLevel(%d, %d) = %d, want %d", tc.days, tc.warn, got, tc.want)
		}
	}
}

func TestDedupeThresholds(t *testing.T) {
	got := dedupeThresholds(14, 7, 7, 3, 0, -1, 14)
	want := []int{14, 7, 3}
	if len(got) != len(want) {
		t.Fatalf("dedupeThresholds len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dedupeThresholds[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestShouldNotifyExpiry(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

	if shouldNotifyExpiry(expiryAlertState{}, false, -1, now) {
		t.Fatal("no alert for level -1")
	}
	if !shouldNotifyExpiry(expiryAlertState{}, false, 14, now) {
		t.Fatal("first warning should notify")
	}
	if shouldNotifyExpiry(expiryAlertState{level: 14, at: now}, true, 14, now) {
		t.Fatal("same bucket should not notify")
	}
	if !shouldNotifyExpiry(expiryAlertState{level: 14, at: now}, true, 7, now) {
		t.Fatal("more urgent bucket should notify")
	}
	if shouldNotifyExpiry(expiryAlertState{level: 0, at: now}, true, 0, now) {
		t.Fatal("expired should not repeat within 24h")
	}
	later := now.Add(25 * time.Hour)
	if !shouldNotifyExpiry(expiryAlertState{level: 0, at: now}, true, 0, later) {
		t.Fatal("expired should repeat after 24h")
	}
}
