package ratelimit

import (
	"testing"
	"time"
)

func TestBuildRateLimitStatus(t *testing.T) {
	now := time.Date(2026, 6, 6, 15, 30, 0, 0, time.UTC)
	status := buildRateLimitStatus(5, DefaultDailyLimit, now)

	if status.Remaining != 15 {
		t.Fatalf("remaining = %d, want 15", status.Remaining)
	}
	if status.Used != 5 {
		t.Fatalf("used = %d, want 5", status.Used)
	}
	if status.ResetsAt != EndOfDayUTC(now) {
		t.Fatalf("resetsAt = %v, want %v", status.ResetsAt, EndOfDayUTC(now))
	}
}

func TestEndOfDayUTC(t *testing.T) {
	now := time.Date(2026, 6, 6, 15, 30, 0, 0, time.UTC)
	end := EndOfDayUTC(now)
	if end != time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("end = %v", end)
	}
}
