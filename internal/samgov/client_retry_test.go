package samgov

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 4, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, MaxElapsed: 500 * time.Millisecond}
}

func TestSearch_Retries5xxThenSucceeds(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"totalRecords":0,"opportunitiesData":[]}`)
	}))
	defer srv.Close()

	c, err := NewClient("k", WithRetryPolicy(fastPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	_, err = c.Search(SearchParams{Limit: 1})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls=%d, want 3 (2 failures + success)", calls.Load())
	}
}

func TestSearch_RetriesOnRateLimitThenGivesUp(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, err := NewClient("a,b", WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond, MaxElapsed: 200 * time.Millisecond}))
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	_, err = c.Search(SearchParams{Limit: 1})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v, want ErrRateLimited", err)
	}
	// 2 keys × 3 Do attempts = 6 underlying HTTP calls.
	if calls.Load() < 4 {
		t.Errorf("calls=%d, expected at least one retry cycle", calls.Load())
	}
}

func TestSearch_RetryAfterHonored(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"totalRecords":0,"opportunitiesData":[]}`)
	}))
	defer srv.Close()

	c, err := NewClient("only-key", WithRetryPolicy(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, MaxElapsed: 5 * time.Second}))
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	start := time.Now()
	if _, err := c.Search(SearchParams{Limit: 1}); err != nil {
		t.Fatalf("Search err=%v", err)
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Errorf("Retry-After=1s not honored; elapsed=%v", time.Since(start))
	}
}

func TestSearch_ContextCancelAborts(t *testing.T) {
	// Server blocks until client disconnects.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c, err := NewClient("k", WithRetryPolicy(fastPolicy()))
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = c.SearchCtx(ctx, SearchParams{Limit: 1})
	if err == nil {
		t.Fatal("expected error on ctx cancel")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("context cancel too slow: %v", time.Since(start))
	}
}

func TestSearch_NetworkErrorRetries(t *testing.T) {
	// Create a server, take its URL, then close it — dial errors are retryable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	c, err := NewClient("k", WithRetryPolicy(RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond, MaxElapsed: 100 * time.Millisecond}))
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = url

	_, err = c.Search(SearchParams{Limit: 1})
	if err == nil {
		t.Fatal("expected error")
	}
	// Dial errors are Retryable; we should not see a bare http get error class mis-propagating.
	if errors.Is(err, context.Canceled) {
		t.Errorf("unexpected ctx error: %v", err)
	}
}
