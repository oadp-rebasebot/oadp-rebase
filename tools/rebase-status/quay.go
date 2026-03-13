package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// QuayClient queries the Quay.io public API for tag metadata.
type QuayClient struct {
	client *http.Client
	cache  map[string]*QuayTagInfo
	mu     sync.Mutex
}

// QuayTagInfo holds tag metadata from the Quay API.
type QuayTagInfo struct {
	Exists       bool
	LastModified time.Time
	Digest       string
}

func NewQuayClient() *QuayClient {
	return &QuayClient{
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  make(map[string]*QuayTagInfo),
	}
}

// TagInfo fetches metadata for a specific tag on a Quay repository.
// namespace/repo is e.g. "konveyor/velero", tag is e.g. "oadp-1.6".
func (q *QuayClient) TagInfo(namespace, repo, tag string) (*QuayTagInfo, error) {
	key := fmt.Sprintf("%s/%s:%s", namespace, repo, tag)

	q.mu.Lock()
	if cached, ok := q.cache[key]; ok {
		q.mu.Unlock()
		return cached, nil
	}
	q.mu.Unlock()

	url := fmt.Sprintf("https://quay.io/api/v1/repository/%s/%s/tag/?specificTag=%s&limit=1",
		namespace, repo, tag)

	resp, err := q.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("quay API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading quay response: %w", err)
	}

	if resp.StatusCode != 200 {
		return &QuayTagInfo{Exists: false}, nil
	}

	var result struct {
		Tags []struct {
			Name         string `json:"name"`
			StartTS      int64  `json:"start_ts"`
			LastModified string `json:"last_modified"`
			Digest       string `json:"manifest_digest"`
		} `json:"tags"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing quay response: %w", err)
	}

	info := &QuayTagInfo{Exists: false}
	if len(result.Tags) > 0 {
		t := result.Tags[0]
		info.Exists = true
		info.LastModified = time.Unix(t.StartTS, 0)
		info.Digest = t.Digest
	}

	q.mu.Lock()
	q.cache[key] = info
	q.mu.Unlock()

	return info, nil
}

// FormatAge returns a human-readable age string like "2d ago", "3h ago".
func FormatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}
