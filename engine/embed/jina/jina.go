// Package jina implements the Jina AI embedding backend.
//
// Supports jina-clip-v2 (multimodal: text + image, 1024d),
// jina-embeddings-v3 (text-only, 1024d with MRL).
package jina

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/embed"
	"github.com/godeps/aigo/engine/httpx"
)

const (
	DefaultModel      = "jina-clip-v2"
	DefaultDimensions = 1024
	defaultBaseURL    = "https://api.jina.ai/v1"
)

// Config configures the Jina embedding engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	RPM        int
	HTTPClient *http.Client
}

// Engine implements embed.EmbedEngine for Jina AI.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// New creates a Jina embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("JINA_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("jina embed: JINA_API_KEY not set")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}
	dims := cfg.Dimensions
	if dims <= 0 {
		dims = DefaultDimensions
	}
	rpm := cfg.RPM
	if rpm <= 0 {
		rpm = 500
	}

	return &Engine{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		dimensions: dims,
		limiter:    embed.NewRateLimiter(rpm),
		client:     httpx.OrDefault(cfg.HTTPClient, 30*time.Second),
	}, nil
}

func (e *Engine) Dimensions() int { return e.dimensions }

// Embed produces a vector for text or image content.
// Jina CLIP v2 supports both text and image in a unified vector space.
func (e *Engine) Embed(ctx context.Context, req embed.Request) (embed.Result, error) {
	if req.Type == embed.ContentVideo {
		return embed.Result{}, fmt.Errorf("jina embed: video content not supported, use text or image")
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	var input apiInput
	switch req.Type {
	case embed.ContentText:
		text, _ := req.Content.(string)
		input = apiInput{Text: text}
	case embed.ContentImage:
		data, _ := req.Content.([]byte)
		input = apiInput{Image: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)}
	}

	var result embed.Result
	err := embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, input)
		return rerr
	}, 3, time.Second)

	return result, err
}

// EmbedBatch embeds multiple requests sequentially.
func (e *Engine) EmbedBatch(ctx context.Context, reqs []embed.Request) ([]embed.Result, error) {
	results := make([]embed.Result, len(reqs))
	for i, req := range reqs {
		r, err := e.Embed(ctx, req)
		if err != nil {
			return results[:i], fmt.Errorf("batch item %d: %w", i, err)
		}
		results[i] = r
	}
	return results, nil
}

// EmbedCapabilities implements embed.Describer.
func (e *Engine) EmbedCapabilities() embed.Capability {
	return embed.Capability{
		SupportedTypes: []embed.ContentType{embed.ContentText, embed.ContentImage},
		Models:         []string{"jina-clip-v2", "jina-embeddings-v3"},
		MaxDimensions:  1024,
		SupportsMRL:    true,
	}
}

func (e *Engine) doEmbed(ctx context.Context, input apiInput) (embed.Result, error) {
	apiReq := apiRequest{
		Model:      e.model,
		Input:      []apiInput{input},
		Dimensions: e.dimensions,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return embed.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return embed.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return embed.Result{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return embed.Result{}, err
	}

	if resp.StatusCode != 200 {
		return embed.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "jina-embed")
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return embed.Result{}, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Data) == 0 || len(apiResp.Data[0].Embedding) == 0 {
		return embed.Result{}, fmt.Errorf("jina embed: empty embedding returned")
	}

	return embed.Result{
		Vector:     apiResp.Data[0].Embedding,
		Dimensions: len(apiResp.Data[0].Embedding),
		Model:      apiResp.Model,
	}, nil
}

// --- API types ---

type apiInput struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type apiRequest struct {
	Model      string     `json:"model"`
	Input      []apiInput `json:"input"`
	Dimensions int        `json:"dimensions,omitempty"`
}

type apiResponse struct {
	Data  []apiEmbeddingData `json:"data"`
	Model string             `json:"model"`
}

type apiEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
