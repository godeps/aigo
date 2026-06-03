// Package pexels implements the material.Searcher interface for the Pexels stock platform.
package pexels

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
	material.Register("pexels", func(p material.ParsedURI) (material.Searcher, error) {
		return New(Config{APIKey: p.APIKey}), nil
	})
}

const (
	photosURL = "https://api.pexels.com/v1/search"
	videosURL = "https://api.pexels.com/videos/search"
)

// Config configures the Pexels searcher.
type Config struct {
	APIKey     string
	RPM        int // requests per minute; default 30
	HTTPClient *http.Client
}

// Searcher queries the Pexels API for photos and videos.
type Searcher struct {
	apiKey  string
	client  *http.Client
	limiter *material.RateLimiter
}

// New creates a Pexels searcher.
// If APIKey is empty, falls back to PEXELS_API_KEY env var.
func New(cfg Config) *Searcher {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("PEXELS_API_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	rpm := cfg.RPM
	if rpm <= 0 {
		rpm = 30
	}
	return &Searcher{apiKey: apiKey, client: client, limiter: material.NewRateLimiter(rpm)}
}

func (s *Searcher) Source() string                { return "pexels" }
func (s *Searcher) SupportedMediaTypes() []string { return []string{"image", "video"} }

// Search queries Pexels for photos and/or videos matching the request.
func (s *Searcher) Search(ctx context.Context, req material.Request) (material.Result, error) {
	if s.apiKey == "" {
		return material.Result{}, fmt.Errorf("pexels: API key not configured")
	}

	wantImage := true
	wantVideo := true
	if len(req.MediaTypes) > 0 {
		wantImage = false
		wantVideo = false
		for _, mt := range req.MediaTypes {
			switch mt {
			case "image":
				wantImage = true
			case "video":
				wantVideo = true
			}
		}
	}

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 20
	}

	var result material.Result
	result.Source = "pexels"

	if wantImage {
		items, total, err := s.searchPhotos(ctx, req.Query, maxResults, req.Page)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, items...)
		result.Total += total
	}

	if wantVideo {
		items, total, err := s.searchVideos(ctx, req.Query, maxResults, req.Page)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, items...)
		result.Total += total
	}

	if maxResults > 0 && len(result.Items) > maxResults {
		result.Items = result.Items[:maxResults]
	}

	return result, nil
}

func (s *Searcher) searchPhotos(ctx context.Context, query string, perPage, page int) ([]material.Item, int, error) {
	params := url.Values{
		"query":    {query},
		"per_page": {strconv.Itoa(perPage)},
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}

	body, err := s.doRequest(ctx, photosURL+"?"+params.Encode())
	if err != nil {
		return nil, 0, err
	}

	var resp photosResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("pexels: parse photos response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Photos))
	for _, p := range resp.Photos {
		items = append(items, material.Item{
			ID:          strconv.Itoa(p.ID),
			URI:         p.URL,
			PreviewURL:  p.Src.Medium,
			DownloadURL: p.Src.Original,
			MediaType:   "image",
			Width:       p.Width,
			Height:      p.Height,
			Source:      "pexels",
			Author:      p.Photographer,
			License:     "Pexels License",
		})
	}
	return items, resp.TotalResults, nil
}

func (s *Searcher) searchVideos(ctx context.Context, query string, perPage, page int) ([]material.Item, int, error) {
	params := url.Values{
		"query":    {query},
		"per_page": {strconv.Itoa(perPage)},
	}
	if page > 0 {
		params.Set("page", strconv.Itoa(page))
	}

	body, err := s.doRequest(ctx, videosURL+"?"+params.Encode())
	if err != nil {
		return nil, 0, err
	}

	var resp videosResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("pexels: parse videos response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Videos))
	for _, v := range resp.Videos {
		var downloadURL string
		var width, height int
		if len(v.VideoFiles) > 0 {
			best := v.VideoFiles[0]
			for _, f := range v.VideoFiles {
				if f.Width > best.Width {
					best = f
				}
			}
			downloadURL = best.Link
			width = best.Width
			height = best.Height
		}
		items = append(items, material.Item{
			ID:          strconv.Itoa(v.ID),
			URI:         v.URL,
			PreviewURL:  v.Image,
			DownloadURL: downloadURL,
			MediaType:   "video",
			Width:       width,
			Height:      height,
			Duration:    float64(v.Duration),
			Source:      "pexels",
			Author:      v.User.Name,
			License:     "Pexels License",
			Tags:        extractVideoTags(v),
		})
	}
	return items, resp.TotalResults, nil
}

func (s *Searcher) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	if err := s.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	var body []byte
	err := material.RetryWithContext(ctx, func() error {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return err
		}
		httpReq.Header.Set("Authorization", s.apiKey)

		resp, err := s.client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("pexels: request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("pexels: read response: %w", err)
		}
		if resp.StatusCode == 429 {
			return fmt.Errorf("pexels: rate limited (HTTP 429)")
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("pexels: server error (HTTP %d)", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("pexels: API error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
		}
		return nil
	}, 3, 500*time.Millisecond)

	return body, err
}

func extractVideoTags(v video) []string {
	if len(v.VideoTags) == 0 {
		return nil
	}
	tags := make([]string, 0, len(v.VideoTags))
	for _, t := range v.VideoTags {
		if t.Name != "" {
			tags = append(tags, t.Name)
		}
	}
	return tags
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// --- API response types ---

type photosResponse struct {
	TotalResults int     `json:"total_results"`
	Photos       []photo `json:"photos"`
}

type photo struct {
	ID           int      `json:"id"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	URL          string   `json:"url"`
	Photographer string   `json:"photographer"`
	Src          photoSrc `json:"src"`
}

type photoSrc struct {
	Original string `json:"original"`
	Large2x  string `json:"large2x"`
	Large    string `json:"large"`
	Medium   string `json:"medium"`
	Small    string `json:"small"`
}

type videosResponse struct {
	TotalResults int     `json:"total_results"`
	Videos       []video `json:"videos"`
}

type video struct {
	ID         int         `json:"id"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	URL        string      `json:"url"`
	Image      string      `json:"image"`
	Duration   int         `json:"duration"`
	User       videoUser   `json:"user"`
	VideoFiles []videoFile `json:"video_files"`
	VideoTags  []videoTag  `json:"video_tags"`
}

type videoUser struct {
	Name string `json:"name"`
}

type videoFile struct {
	ID       int    `json:"id"`
	Quality  string `json:"quality"`
	FileType string `json:"file_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Link     string `json:"link"`
}

type videoTag struct {
	Name string `json:"name"`
}

// Ensure interface compliance.
var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
