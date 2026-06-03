// Package pixabay implements the material.Searcher interface for the Pixabay stock platform.
package pixabay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/godeps/aigo/material"
)

func init() {
	material.Register("pixabay", func(p material.ParsedURI) (material.Searcher, error) {
		return New(Config{APIKey: p.APIKey}), nil
	})
}

const (
	imagesURL = "https://pixabay.com/api/"
	videosURL = "https://pixabay.com/api/videos/"
)

// Config configures the Pixabay searcher.
type Config struct {
	APIKey     string
	HTTPClient *http.Client
}

// Searcher queries the Pixabay API for images and videos.
type Searcher struct {
	apiKey string
	client *http.Client
}

// New creates a Pixabay searcher.
// If APIKey is empty, falls back to PIXABAY_API_KEY env var.
func New(cfg Config) *Searcher {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("PIXABAY_API_KEY")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Searcher{apiKey: apiKey, client: client}
}

func (s *Searcher) Source() string                { return "pixabay" }
func (s *Searcher) SupportedMediaTypes() []string { return []string{"image", "video"} }

// Search queries Pixabay for images and/or videos matching the request.
func (s *Searcher) Search(ctx context.Context, req material.Request) (material.Result, error) {
	if s.apiKey == "" {
		return material.Result{}, fmt.Errorf("pixabay: API key not configured")
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

	perPage := req.MaxResults
	if perPage <= 0 {
		perPage = 20
	}

	var result material.Result
	result.Source = "pixabay"

	if wantImage {
		items, total, err := s.searchImages(ctx, req, perPage)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, items...)
		result.Total += total
	}

	if wantVideo {
		items, total, err := s.searchVideos(ctx, req, perPage)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, items...)
		result.Total += total
	}

	if req.MaxResults > 0 && len(result.Items) > req.MaxResults {
		result.Items = result.Items[:req.MaxResults]
	}

	return result, nil
}

func (s *Searcher) searchImages(ctx context.Context, req material.Request, perPage int) ([]material.Item, int, error) {
	params := url.Values{
		"key":      {s.apiKey},
		"q":        {req.Query},
		"per_page": {strconv.Itoa(perPage)},
	}
	if req.Page > 0 {
		params.Set("page", strconv.Itoa(req.Page))
	}
	if req.Sort == "newest" {
		params.Set("order", "latest")
	}
	if req.Locale != "" {
		params.Set("lang", req.Locale)
	}

	body, err := s.doRequest(ctx, imagesURL+"?"+params.Encode())
	if err != nil {
		return nil, 0, err
	}

	var resp imageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("pixabay: parse images response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		var tags []string
		if h.Tags != "" {
			for _, t := range strings.Split(h.Tags, ",") {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}
		items = append(items, material.Item{
			ID:          strconv.Itoa(h.ID),
			URI:         h.PageURL,
			PreviewURL:  h.WebformatURL,
			DownloadURL: h.LargeImageURL,
			MediaType:   "image",
			Width:       h.ImageWidth,
			Height:      h.ImageHeight,
			Size:        int64(h.ImageSize),
			Source:      "pixabay",
			Author:      h.User,
			License:     "Pixabay License",
			Tags:        tags,
		})
	}
	return items, resp.TotalHits, nil
}

func (s *Searcher) searchVideos(ctx context.Context, req material.Request, perPage int) ([]material.Item, int, error) {
	params := url.Values{
		"key":      {s.apiKey},
		"q":        {req.Query},
		"per_page": {strconv.Itoa(perPage)},
	}
	if req.Page > 0 {
		params.Set("page", strconv.Itoa(req.Page))
	}
	if req.Sort == "newest" {
		params.Set("order", "latest")
	}
	if req.Locale != "" {
		params.Set("lang", req.Locale)
	}

	body, err := s.doRequest(ctx, videosURL+"?"+params.Encode())
	if err != nil {
		return nil, 0, err
	}

	var resp videoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("pixabay: parse videos response: %w", err)
	}

	items := make([]material.Item, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		var tags []string
		if h.Tags != "" {
			for _, t := range strings.Split(h.Tags, ",") {
				if trimmed := strings.TrimSpace(t); trimmed != "" {
					tags = append(tags, trimmed)
				}
			}
		}
		var downloadURL string
		var width, height int
		if large, ok := h.Videos["large"]; ok {
			downloadURL = large.URL
			width = large.Width
			height = large.Height
		} else if medium, ok := h.Videos["medium"]; ok {
			downloadURL = medium.URL
			width = medium.Width
			height = medium.Height
		}
		items = append(items, material.Item{
			ID:          strconv.Itoa(h.ID),
			URI:         h.PageURL,
			PreviewURL:  h.PictureID,
			DownloadURL: downloadURL,
			MediaType:   "video",
			Width:       width,
			Height:      height,
			Duration:    float64(h.Duration),
			Source:      "pixabay",
			Author:      h.User,
			License:     "Pixabay License",
			Tags:        tags,
		})
	}
	return items, resp.TotalHits, nil
}

func (s *Searcher) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("pixabay: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pixabay: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pixabay: API error (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// --- API response types ---

type imageResponse struct {
	Total     int        `json:"total"`
	TotalHits int        `json:"totalHits"`
	Hits      []imageHit `json:"hits"`
}

type imageHit struct {
	ID            int    `json:"id"`
	PageURL       string `json:"pageURL"`
	Tags          string `json:"tags"`
	WebformatURL  string `json:"webformatURL"`
	LargeImageURL string `json:"largeImageURL"`
	ImageWidth    int    `json:"imageWidth"`
	ImageHeight   int    `json:"imageHeight"`
	ImageSize     int    `json:"imageSize"`
	User          string `json:"user"`
}

type videoResponse struct {
	Total     int        `json:"total"`
	TotalHits int        `json:"totalHits"`
	Hits      []videoHit `json:"hits"`
}

type videoHit struct {
	ID        int                  `json:"id"`
	PageURL   string               `json:"pageURL"`
	Tags      string               `json:"tags"`
	PictureID string               `json:"picture_id"`
	Duration  int                  `json:"duration"`
	User      string               `json:"user"`
	Videos    map[string]videoSize `json:"videos"`
}

type videoSize struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Size   int    `json:"size"`
}

var (
	_ material.Searcher  = (*Searcher)(nil)
	_ material.Describer = (*Searcher)(nil)
)
