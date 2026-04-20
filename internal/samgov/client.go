package samgov

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var ErrRateLimited = errors.New("rate limited: all API keys exhausted")

type Client struct {
	keys        []string
	current     atomic.Int64
	http        *http.Client
	baseURL     string
	retryPolicy RetryPolicy
}

type ClientOption func(*Client)

func WithRetryPolicy(p RetryPolicy) ClientOption {
	return func(c *Client) { c.retryPolicy = p }
}

func WithHTTPClient(h *http.Client) ClientOption {
	return func(c *Client) { c.http = h }
}

func NewClient(apiKeyEnv string, opts ...ClientOption) (*Client, error) {
	if apiKeyEnv == "" {
		return nil, errors.New("SAMGOV_API_KEY is required")
	}
	var keys []string
	for k := range strings.SplitSeq(apiKeyEnv, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no valid API keys found")
	}
	c := &Client{
		keys:        keys,
		http:        &http.Client{Timeout: 30 * time.Second},
		baseURL:     "https://api.sam.gov/opportunities/v2/search",
		retryPolicy: DefaultRetryPolicy,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) currentKey() string {
	idx := c.current.Load() % int64(len(c.keys))
	return c.keys[idx]
}

func (c *Client) rotateKey() {
	c.current.Add(1)
}

// Search is a backwards-compatible wrapper around SearchCtx.
func (c *Client) Search(params SearchParams) (*APIResponse, error) {
	return c.SearchCtx(context.Background(), params)
}

// SearchCtx performs a single search call with retries, backoff, and key rotation.
func (c *Client) SearchCtx(ctx context.Context, params SearchParams) (*APIResponse, error) {
	var resp *APIResponse
	err := Do(ctx, c.retryPolicy, func(ctx context.Context) error {
		r, err := c.searchOnce(ctx, params)
		if err != nil {
			return err
		}
		resp = r
		return nil
	})
	return resp, err
}

// searchOnce executes a single logical search, cycling through keys on 401/403/429
// until either success, a non-retryable error, or all keys fail. If all keys fail
// within this cycle, it returns a Retryable ErrRateLimited so the outer Do loop
// can back off and try again (honoring Retry-After when seen).
func (c *Client) searchOnce(ctx context.Context, params SearchParams) (*APIResponse, error) {
	startIdx := c.current.Load()
	var retryAfter time.Duration

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		u, _ := url.Parse(c.baseURL)
		q := u.Query()
		q.Set("api_key", c.currentKey())
		q.Set("limit", fmt.Sprintf("%d", params.Limit))
		q.Set("offset", fmt.Sprintf("%d", params.Offset))

		if params.NoticeID != "" {
			q.Set("noticeid", params.NoticeID)
		} else {
			if params.PostedFrom != "" {
				q.Set("postedFrom", params.PostedFrom)
			}
			if params.PostedTo != "" {
				q.Set("postedTo", params.PostedTo)
			}
		}
		if params.Title != "" {
			q.Set("title", params.Title)
		}
		if params.Type != "" {
			q.Set("ptype", params.Type)
		}
		if params.NAICS != "" {
			q.Set("ncode", params.NAICS)
		}
		if params.State != "" {
			q.Set("state", params.State)
		}
		if params.SetAside != "" {
			q.Set("typeOfSetAside", params.SetAside)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, Retryable(fmt.Errorf("http get: %w", err))
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, Retryable(fmt.Errorf("read body: %w", err))
		}

		if resp.StatusCode == 429 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			if ra := parseRetryAfter(resp.Header.Get("Retry-After")); ra > 0 {
				retryAfter = ra
			}
			c.rotateKey()
			if c.current.Load()%int64(len(c.keys)) == startIdx%int64(len(c.keys)) {
				if retryAfter > 0 {
					return nil, RetryableAfter(ErrRateLimited, retryAfter)
				}
				return nil, Retryable(ErrRateLimited)
			}
			continue
		}

		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			return nil, Retryable(fmt.Errorf("api error %d: %s", resp.StatusCode, truncate(string(body), 200)))
		}

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
		}

		var apiResp APIResponse
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		return &apiResp, nil
	}
}

func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(h)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

type WindowResult struct {
	TotalFetched int
	APICalls     int
	RateLimited  bool
}

func (c *Client) SearchWindow(from, to string, onPage func([]map[string]any) error) (*WindowResult, error) {
	return c.SearchWindowCtx(context.Background(), from, to, onPage)
}

func (c *Client) SearchWindowCtx(ctx context.Context, from, to string, onPage func([]map[string]any) error) (*WindowResult, error) {
	offset := 0
	totalFetched := 0
	apiCalls := 0

	for {
		if err := ctx.Err(); err != nil {
			return &WindowResult{TotalFetched: totalFetched, APICalls: apiCalls}, err
		}
		apiCalls++
		resp, err := c.SearchCtx(ctx, SearchParams{
			Limit:      1000,
			Offset:     offset,
			PostedFrom: from,
			PostedTo:   to,
		})
		if errors.Is(err, ErrRateLimited) {
			return &WindowResult{TotalFetched: totalFetched, APICalls: apiCalls, RateLimited: true}, nil
		}
		if err != nil {
			return nil, err
		}

		pageCount := len(resp.OpportunitiesData)
		if pageCount > 0 {
			if err := onPage(resp.OpportunitiesData); err != nil {
				return nil, fmt.Errorf("onPage: %w", err)
			}
		}
		totalFetched += pageCount

		totalRecords := int64(0)
		if resp.TotalRecords != nil {
			totalRecords = *resp.TotalRecords
		}
		if int64(totalFetched) >= totalRecords || pageCount < 1000 {
			break
		}
		offset += 1000
	}

	return &WindowResult{TotalFetched: totalFetched, APICalls: apiCalls}, nil
}
