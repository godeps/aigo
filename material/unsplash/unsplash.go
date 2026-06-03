// Package unsplash implements the material.Searcher interface for the Unsplash photo platform.
package unsplash

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/godeps/aigo/material"
)

func init() {
	material.Register("unsplash", func(p material.ParsedURI) (material.Searcher, error) {
		return New(Config{AccessKey: p.APIKey}), nil
	})
}

const baseURL = "https://api.unsplash.com/search/photos"

// Config configures the Unsplash searcher.
type Config struct {
	AccessKey  string
	HTTPClient *http.Client
}

// Searcher queries the Unsplash API for photos.
type Searcher struct {
	accessKey string
	client    *http.Client
}

// New creates an Unsplash searcher.
// If AccessKey is empty, falls back to UNSPLASH_ACCESS_KEY env var.
func New(cfg Config) *Searcher {
	accessKey := cfg.AccessKey
	if accessKey == "" {
		accessKey = os.Getenv("UNSPLASH_ACCESS_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Searcher{accessKey: accessKey, client: client}
}

func (s *Searcher) Source() string                { return "unsplash" }
func (s *Searcher) SupportedMediaTypes() []string { return []string{"image"} }

// Search queries Unsplash for photos matching the request.
func (s *Searcher) Search(ctx context.Context, req material.Request) (material.Result, error) {
	if s.accessKey == "" {
		return material.Result{}, fmt.Errorf("unsplash: access key not configured")
	}

	if len(req.MediaTypes) > 0 {
		hasImage := false
		for _, mt := range req.MediaTypes {
			if mt == "image" {
				hasImage = true
				break
			}
		}
		if !hasImage {
			return material.Result{Source: "unsplash"}, nil
		}
	}

	perPage := req.MaxResults
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 30 {
		perPage = 30
	}

	params := url.Values{
		"query":    {req.Query},
		"per_page": {strconv.Itoa(perPage)},
	}
	if req.Page > 0 {
		params.Set("page", strconv.Itoa(req.Page))
	}
	if req.Sort != "" {
		orderBy := "relevant"
		if req.Sort == "newest" {
			orderBy = "latest"
		}
		params.Set("order_by", orderBy)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return material.Result{}, err
	}
	httpReq.Header.Set("Authorization", "Client-ID "+s.accessKey)
	httpReq.Header.Set("Accept-Version", "v1")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return material.Result{}, fmt.Errorf("unsplash: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return material.Result{}, fmt.Errorf("unsplash: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return material.Result{}, fmt.Errorf("unsplash: API error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return material.Result{}, fmt.Errorf("unsplash: parse response: %w", err)
	}

	items := make([]material.Item, 0, len(apiResp.Results))
	for _, p := range apiResp.Results {
		var tags []string
		for _, t := range p.Tags {
			if t.Title != "" {
				tags = append(tags, t.Title)
			}
		}
		items = append(items, material.Item{
			ID:          p.ID,
			URI:         p.Links.HTML,
			PreviewURL:  p.URLs.Small,
			DownloadURL: p.URLs.Full,
			MediaType:   "image",
			Width:       p.Width,
			Height:      p.Height,
			Source:      "unsplash",
			Author:      p.User.Name,
			License:     "Unsplash License",
			Caption:     p.Description,
			Tags:        tags,
		})
	}

	return material.Result{
		Items:  items,
		Total:  apiResp.Total,
		Source: "unsplash",
	}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// --- API response types ---

type apiResponse struct {
	Total      int      `json:"total"`
	TotalPages int      `json:"total_pages"`
	Results    []result `json:"results"`
}

type result struct {
	ID          string     `json:"id"`
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	Description string     `json:"description"`
	URLs        resultURLs `json:"urls"`
	Links       links      `json:"links"`
	User        user       `json:"user"`
	Tags        []tag      `json:"tags"`
}

type resultURLs struct {
	Raw     string `json:"raw"`
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

type links struct {
	HTML     string `json:"html"`
	Download string `json:"download"`
}

type user struct {
	Name string `json:"name"`
}

type tag struct {
	Title string `json:"title"`
}

var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
