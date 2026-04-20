package samgov

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryPolicy governs exponential backoff with full jitter.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	MaxElapsed  time.Duration
}

var DefaultRetryPolicy = RetryPolicy{
	MaxAttempts: 5,
	BaseDelay:   1 * time.Second,
	MaxDelay:    30 * time.Second,
	MaxElapsed:  2 * time.Minute,
}

// ErrRetryable wraps an error to mark it retryable by Do.
type retryableErr struct{ err error }

func (r retryableErr) Error() string { return r.err.Error() }
func (r retryableErr) Unwrap() error { return r.err }

// Retryable marks an error as transient so Do will retry.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableErr{err: err}
}

// IsRetryable reports whether err was wrapped by Retryable.
func IsRetryable(err error) bool {
	var r retryableErr
	return errors.As(err, &r)
}

// RetryHint lets a retryable error suggest an explicit delay (e.g. Retry-After).
type retryHinted struct {
	err   error
	delay time.Duration
}

func (r retryHinted) Error() string { return r.err.Error() }
func (r retryHinted) Unwrap() error { return r.err }

// RetryableAfter marks err retryable and requests the given delay before next attempt.
func RetryableAfter(err error, d time.Duration) error {
	if err == nil {
		return nil
	}
	return retryHinted{err: retryableErr{err: err}, delay: d}
}

// Do runs fn with retries. fn should return Retryable(err) / RetryableAfter(err, d)
// for transient errors; any other error terminates the loop and is returned.
func Do(ctx context.Context, p RetryPolicy, fn func(ctx context.Context) error) error {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 1
	}
	start := time.Now()
	var lastErr error
	for attempt := 0; attempt < p.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return err
		}
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsRetryable(err) {
			return err
		}
		if p.MaxElapsed > 0 && time.Since(start) >= p.MaxElapsed {
			return err
		}
		if attempt == p.MaxAttempts-1 {
			return err
		}

		delay := backoff(p, attempt)
		var hinted retryHinted
		if errors.As(err, &hinted) && hinted.delay > 0 {
			// Honor explicit server-provided hint. MaxElapsed still bounds total wall time.
			delay = hinted.delay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

func backoff(p RetryPolicy, attempt int) time.Duration {
	base := p.BaseDelay
	if base <= 0 {
		base = time.Second
	}
	max := p.MaxDelay
	if max <= 0 {
		max = 30 * time.Second
	}
	exp := base << attempt
	if exp <= 0 || exp > max {
		exp = max
	}
	// Full jitter: random in [0, exp]
	return time.Duration(rand.Int63n(int64(exp) + 1))
}
