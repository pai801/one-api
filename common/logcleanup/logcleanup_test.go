package logcleanup

import (
	"os"
	"testing"
	"time"
)

func TestRetentionDaysFromEnv(t *testing.T) {
	t.Setenv("LOG_CLEAN_DAYS", " 3 ")
	if got := RetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}

	t.Setenv("LOG_CLEAN_DAYS", "0")
	if got := RetentionDays(); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}

	t.Setenv("LOG_CLEAN_DAYS", "3")
	if got := RetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}

	t.Setenv("LOG_CLEAN_DAYS", "-1")
	if got := RetentionDays(); got != 7 {
		t.Fatalf("got %d, want 7", got)
	}

	t.Setenv("LOG_CLEAN_DAYS", "8")
	if got := RetentionDays(); got != 7 {
		t.Fatalf("got %d, want 7", got)
	}

	t.Setenv("LOG_CLEAN_DAYS", "abc")
	if got := RetentionDays(); got != 7 {
		t.Fatalf("got %d, want 7", got)
	}
}

func TestBodiesRetentionDaysFromEnv(t *testing.T) {
	t.Setenv("LOG_CLEAN_BODIES_DAYS", " 2 ")
	if got := BodiesRetentionDays(); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_DAYS", "0")
	if got := BodiesRetentionDays(); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_DAYS", "2")
	if got := BodiesRetentionDays(); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_DAYS", "-1")
	if got := BodiesRetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_DAYS", "4")
	if got := BodiesRetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}

	t.Setenv("LOG_CLEAN_BODIES_DAYS", "abc")
	if got := BodiesRetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}

	orig, hadOrig := os.LookupEnv("LOG_CLEAN_BODIES_DAYS")
	os.Unsetenv("LOG_CLEAN_BODIES_DAYS")
	defer func() {
		if hadOrig {
			os.Setenv("LOG_CLEAN_BODIES_DAYS", orig)
		} else {
			os.Unsetenv("LOG_CLEAN_BODIES_DAYS")
		}
	}()
	if got := BodiesRetentionDays(); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}

func TestNextUTCMidnight(t *testing.T) {
	now := time.Date(2026, 6, 15, 13, 45, 0, 0, time.FixedZone("UTC+8", 8*3600))
	got := NextUTCMidnight(now)
	want := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCutoffTimestampUsesUTCDayBoundary(t *testing.T) {
	now := time.Date(2026, 6, 15, 18, 30, 0, 0, time.UTC)
	got := CutoffTimestamp(now, 7)
	want := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}

func TestCutoffTimestampConvertsNowToUTCDate(t *testing.T) {
	now := time.Date(2026, 6, 15, 3, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	got := CutoffTimestamp(now, 7)
	want := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
