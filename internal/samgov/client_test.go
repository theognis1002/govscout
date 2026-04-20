package samgov

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func TestNewClient_EmptyKey(t *testing.T) {
	if _, err := NewClient(""); err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestNewClient_OnlyWhitespaceAndCommas(t *testing.T) {
	// Guards against the subtle case where `" , , "` passes the empty-string
	// check but produces zero valid keys after trimming.
	if _, err := NewClient(" , , "); err == nil {
		t.Fatal("expected error for all-empty comma-separated input, got nil")
	}
}

func TestNewClient_MultipleKeys_TrimmedAndEmptiesDropped(t *testing.T) {
	c, err := NewClient(" a , ,b,c, ")
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	wantKeys := []string{"a", "b", "c"}
	if len(c.keys) != len(wantKeys) {
		t.Fatalf("keys = %v, want %v", c.keys, wantKeys)
	}
	for i, k := range wantKeys {
		if c.keys[i] != k {
			t.Errorf("keys[%d] = %q, want %q", i, c.keys[i], k)
		}
	}
}

func TestClient_RotateKey_AdvancesAndWraps(t *testing.T) {
	c, err := NewClient("k1,k2,k3")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.currentKey(); got != "k1" {
		t.Errorf("initial currentKey = %q, want k1", got)
	}
	c.rotateKey()
	if got := c.currentKey(); got != "k2" {
		t.Errorf("after 1 rotate = %q, want k2", got)
	}
	c.rotateKey()
	if got := c.currentKey(); got != "k3" {
		t.Errorf("after 2 rotates = %q, want k3", got)
	}
	c.rotateKey()
	if got := c.currentKey(); got != "k1" {
		t.Errorf("after 3 rotates (wrap) = %q, want k1", got)
	}
}

func TestClient_Search_RateLimitRotatesThroughAllKeysThenFails(t *testing.T) {
	var seenKeys []string
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		seenKeys = append(seenKeys, r.URL.Query().Get("api_key"))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, err := NewClient("k1,k2,k3")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	c.retryPolicy = RetryPolicy{MaxAttempts: 1}

	_, err = c.Search(SearchParams{Limit: 10})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("expected 3 attempts (one per key), got %d", calls.Load())
	}
	// Every key must have been tried exactly once before giving up.
	want := map[string]bool{"k1": true, "k2": true, "k3": true}
	got := map[string]bool{}
	for _, k := range seenKeys {
		got[k] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("key %q was never attempted; seen=%v", k, seenKeys)
		}
	}
}

func TestClient_Search_RateLimitSingleKey(t *testing.T) {
	// Single-key client: after one 429, rotate wraps to startIdx → ErrRateLimited.
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c, err := NewClient("only")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL
	c.retryPolicy = RetryPolicy{MaxAttempts: 1}

	_, err = c.Search(SearchParams{Limit: 10})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("expected exactly 1 call before giving up, got %d", calls.Load())
	}
}

func TestClient_Search_WiresQueryParams(t *testing.T) {
	var gotQuery map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = map[string]string{}
		for k, v := range r.URL.Query() {
			gotQuery[k] = v[0]
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"totalRecords": 2, "opportunitiesData": [{"id":"a"},{"id":"b"}]}`)
	}))
	defer srv.Close()

	c, err := NewClient("my-key")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = srv.URL

	resp, err := c.Search(SearchParams{
		Limit:      50,
		Offset:     100,
		PostedFrom: "01/01/2026",
		PostedTo:   "01/31/2026",
		Title:      "cyber",
		Type:       "k",
		NAICS:      "541511",
		State:      "VA",
		SetAside:   "SBA",
	})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	want := map[string]string{
		"api_key":        "my-key",
		"limit":          "50",
		"offset":         "100",
		"postedFrom":     "01/01/2026",
		"postedTo":       "01/31/2026",
		"title":          "cyber",
		"ptype":          "k",
		"ncode":          "541511",
		"state":          "VA",
		"typeOfSetAside": "SBA",
	}
	for k, v := range want {
		if gotQuery[k] != v {
			t.Errorf("query param %q = %q, want %q", k, gotQuery[k], v)
		}
	}

	if resp.TotalRecords == nil || *resp.TotalRecords != 2 {
		t.Errorf("TotalRecords = %v, want 2", resp.TotalRecords)
	}
	if len(resp.OpportunitiesData) != 2 {
		t.Errorf("got %d opps, want 2", len(resp.OpportunitiesData))
	}
}

func TestClient_Search_NoticeIDSuppressesDateWindow(t *testing.T) {
	// When NoticeID is set, postedFrom/postedTo must NOT be sent — the API
	// returns "no results" if you combine them. This is a real correctness
	// trap: silent behavior regressions here break single-opportunity lookup.
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		fmt.Fprint(w, `{"totalRecords":0,"opportunitiesData":[]}`)
	}))
	defer srv.Close()

	c, _ := NewClient("k")
	c.baseURL = srv.URL

	_, err := c.Search(SearchParams{
		Limit:      1,
		NoticeID:   "abc123",
		PostedFrom: "01/01/2026",
		PostedTo:   "01/31/2026",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("noticeid") != "abc123" {
		t.Errorf("noticeid = %q, want abc123", gotQuery.Get("noticeid"))
	}
	if _, has := gotQuery["postedFrom"]; has {
		t.Error("postedFrom should be suppressed when NoticeID is set")
	}
	if _, has := gotQuery["postedTo"]; has {
		t.Error("postedTo should be suppressed when NoticeID is set")
	}
}

func TestClient_Search_NonRateLimitErrorReturnsAPIError(t *testing.T) {
	// 500 is NOT one of the rotation-triggering statuses — it should surface
	// as a plain error, not trigger rotation.
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := NewClient("k1,k2")
	c.baseURL = srv.URL
	c.retryPolicy = RetryPolicy{MaxAttempts: 1}

	_, err := c.Search(SearchParams{Limit: 1})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Error("500 must not be treated as rate-limited")
	}
	if calls.Load() != 1 {
		t.Errorf("500 should not trigger rotation; got %d calls", calls.Load())
	}
}
