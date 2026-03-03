package samgov

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var ErrRateLimited = errors.New("rate limited: all API keys exhausted")

type Client struct {
	keys    []string
	current atomic.Int64
	http    *http.Client
}

func NewClient(apiKeyEnv string) (*Client, error) {
	if apiKeyEnv == "" {
		return nil, errors.New("SAMGOV_API_KEY is required")
	}
	var keys []string
	for _, k := range strings.Split(apiKeyEnv, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, errors.New("no valid API keys found")
	}
	return &Client{
		keys: keys,
		http: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) currentKey() string {
	idx := c.current.Load() % int64(len(c.keys))
	return c.keys[idx]
}

func (c *Client) rotateKey() {
	c.current.Add(1)
}

func (c *Client) Search(params SearchParams) (*APIResponse, error) {
	startIdx := c.current.Load()

	for {
		u, _ := url.Parse("https://api.sam.gov/opportunities/v2/search")
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

		resp, err := c.http.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("http get: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}

		if resp.StatusCode == 429 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			c.rotateKey()
			if c.current.Load()%int64(len(c.keys)) == startIdx%int64(len(c.keys)) {
				return nil, ErrRateLimited
			}
			continue
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

type WindowResult struct {
	TotalFetched int
	APICalls     int
	RateLimited  bool
}

func (c *Client) SearchWindow(from, to string, onPage func([]map[string]any) error) (*WindowResult, error) {
	offset := 0
	totalFetched := 0
	apiCalls := 0

	for {
		apiCalls++
		resp, err := c.Search(SearchParams{
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
