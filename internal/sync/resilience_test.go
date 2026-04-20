package sync

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/theognis1002/govscout/internal/db"
	"github.com/theognis1002/govscout/internal/samgov"
)

func newTestClient(t *testing.T, baseURL string) *samgov.Client {
	t.Helper()
	c, err := samgov.NewClient("test-key", samgov.WithRetryPolicy(samgov.RetryPolicy{
		MaxAttempts: 2,
		BaseDelay:   time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		MaxElapsed:  100 * time.Millisecond,
	}))
	if err != nil {
		t.Fatal(err)
	}
	samgov.SetBaseURLForTest(c, baseURL)
	return c
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestRunCtx_RecordsCursorAfterWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totalRecords":0,"opportunitiesData":[]}`)
	}))
	defer srv.Close()

	database := openTestDB(t)
	client := newTestClient(t, srv.URL)

	if err := RunCtx(context.Background(), database, client, Options{MaxCalls: 5}); err != nil {
		t.Fatalf("RunCtx: %v", err)
	}

	cursor, err := db.GetSyncState(database, "backfill_cursor")
	if err != nil {
		t.Fatal(err)
	}
	if cursor == "" {
		t.Error("expected backfill_cursor to be persisted")
	}
}

func TestRunCtx_ContextCancelStops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totalRecords":0,"opportunitiesData":[]}`)
	}))
	defer srv.Close()

	database := openTestDB(t)
	client := newTestClient(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := RunCtx(ctx, database, client, Options{MaxCalls: 10}); err == nil {
		t.Fatal("expected ctx error")
	}
}

func TestRunCtx_RateLimitedIncrementalStopsGracefully(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	database := openTestDB(t)
	client := newTestClient(t, srv.URL)

	if err := RunCtx(context.Background(), database, client, Options{MaxCalls: 5}); err != nil {
		t.Fatalf("expected nil on rate limit (graceful), got %v", err)
	}
}
