package logcleanup

import (
	"os"
	"testing"
	"time"
)

func TestRetentionHoursFromEnv(t *testing.T) {
	t.Setenv("LOG_CLEAN_HOURS", " 24 ")
	if got := RetentionHours(); got != 24 {
		t.Fatalf("got %d, want 24", got)
	}

	t.Setenv("LOG_CLEAN_HOURS", "0")
	if got := RetentionHours(); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}

	t.Setenv("LOG_CLEAN_HOURS", "48")
	if got := RetentionHours(); got != 48 {
		t.Fatalf("got %d, want 48", got)
	}

	t.Setenv("LOG_CLEAN_HOURS", "-1")
	if got := RetentionHours(); got != 168 {
		t.Fatalf("got %d, want 168", got)
	}

	t.Setenv("LOG_CLEAN_HOURS", "200")
	if got := RetentionHours(); got != 168 {
		t.Fatalf("got %d, want 168", got)
	}

	t.Setenv("LOG_CLEAN_HOURS", "abc")
	if got := RetentionHours(); got != 168 {
		t.Fatalf("got %d, want 168", got)
	}
}

func TestBodiesRetentionHoursFromEnv(t *testing.T) {
	t.Setenv("LOG_CLEAN_BODIES_HOURS", " 12 ")
	if got := BodiesRetentionHours(); got != 12 {
		t.Fatalf("got %d, want 12", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_HOURS", "0")
	if got := BodiesRetentionHours(); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_HOURS", "24")
	if got := BodiesRetentionHours(); got != 24 {
		t.Fatalf("got %d, want 24", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_HOURS", "-1")
	if got := BodiesRetentionHours(); got != 72 {
		t.Fatalf("got %d, want 72", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_HOURS", "80")
	if got := BodiesRetentionHours(); got != 72 {
		t.Fatalf("got %d, want 72", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_HOURS", "abc")
	if got := BodiesRetentionHours(); got != 72 {
		t.Fatalf("got %d, want 72", got)
	}

	orig, hadOrig := os.LookupEnv("LOG_CLEAN_BODIES_HOURS")
	os.Unsetenv("LOG_CLEAN_BODIES_HOURS")
	defer func() {
		if hadOrig {
			os.Setenv("LOG_CLEAN_BODIES_HOURS", orig)
		} else {
			os.Unsetenv("LOG_CLEAN_BODIES_HOURS")
		}
	}()
	if got := BodiesRetentionHours(); got != 72 {
		t.Fatalf("got %d, want 72", got)
	}
}

func TestNextUTCHour(t *testing.T) {
	now := time.Date(2026, 6, 15, 13, 45, 0, 0, time.FixedZone("UTC+8", 8*3600))
	got := NextUTCHour(now)
	want := time.Date(2026, 6, 15, 6, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCutoffTimestampUsesUTCHourBoundary(t *testing.T) {
	now := time.Date(2026, 6, 15, 18, 30, 0, 0, time.UTC)
	got := CutoffTimestamp(now, 7)
	want := time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestCutoffTimestampConvertsNowToUTCHour(t *testing.T) {
	now := time.Date(2026, 6, 15, 3, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	got := CutoffTimestamp(now, 7)
	want := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
