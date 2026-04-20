package samgov

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_SucceedsFirstTry(t *testing.T) {
	calls := 0
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}, func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls=%d, want 1", calls)
	}
}

func TestDo_RetriesOnRetryable(t *testing.T) {
	calls := 0
	sentinel := errors.New("boom")
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return Retryable(sentinel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls=%d, want 3", calls)
	}
}

func TestDo_NonRetryableStopsImmediately(t *testing.T) {
	calls := 0
	sentinel := errors.New("fatal")
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond}, func(ctx context.Context) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v, want sentinel", err)
	}
	if calls != 1 {
		t.Errorf("calls=%d, want 1 (non-retryable)", calls)
	}
}

func TestDo_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	sentinel := errors.New("keeps failing")
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		return Retryable(sentinel)
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v, want sentinel", err)
	}
	if calls != 3 {
		t.Errorf("calls=%d, want 3", calls)
	}
}

func TestDo_ContextCancelStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	err := Do(ctx, RetryPolicy{MaxAttempts: 5, BaseDelay: 10 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		return Retryable(errors.New("x"))
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
	if calls != 0 {
		t.Errorf("calls=%d, want 0 (ctx cancelled before first attempt)", calls)
	}
}

func TestDo_MaxElapsedBounds(t *testing.T) {
	calls := 0
	start := time.Now()
	err := Do(context.Background(), RetryPolicy{
		MaxAttempts: 100,
		BaseDelay:   5 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		MaxElapsed:  30 * time.Millisecond,
	}, func(ctx context.Context) error {
		calls++
		return Retryable(errors.New("transient"))
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Do ran for %v; MaxElapsed should cap it", elapsed)
	}
	if calls < 2 {
		t.Errorf("expected at least 2 attempts before elapsed cap, got %d", calls)
	}
}

func TestDo_HonorsRetryAfterHint(t *testing.T) {
	calls := 0
	start := time.Now()
	err := Do(context.Background(), RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 100 * time.Millisecond}, func(ctx context.Context) error {
		calls++
		if calls == 1 {
			return RetryableAfter(errors.New("slow down"), 40*time.Millisecond)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 35*time.Millisecond {
		t.Errorf("Retry-After hint not honored; elapsed=%v", time.Since(start))
	}
}
