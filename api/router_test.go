package api

import (
	"testing"
	"time"
)

func TestFormatValueTimestamp(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 17, 12, 30, 0, 123456789, time.FixedZone("EDT", -4*60*60))
	want := "2026-08-17T16:30:00.123456789Z"
	if got := formatValueTimestamp(updatedAt); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if got := formatValueTimestamp(time.Time{}); got != "" {
		t.Fatalf("expected empty timestamp for zero time, got %q", got)
	}
}
