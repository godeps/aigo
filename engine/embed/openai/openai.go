// Package openai implements the OpenAI text embedding backend.
//
// Supports text-embedding-3-small (1536d), text-embedding-3-large (3072d),
// and text-embedding-ada-002 (1536d). All support MRL dimension truncation.
package openai

import (
	"bytes"
	"context"
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
	DefaultModel      = "text-embedding-3-small"
	DefaultDimensions = 1536
	defaultBaseURL    = "https://api.openai.com/v1"
)

// Config configures the OpenAI embedding engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	RPM        int
	HTTPClient *http.Client
}

// Engine implements embed.EmbedEngine for OpenAI embeddings.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// New creates an OpenAI embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("openai embed: OPENAI_API_KEY not set")
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
		rpm = 3000
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

// Embed produces a vector for text content.
func (e *Engine) Embed(ctx context.Context, req embed.Request) (embed.Result, error) {
	if req.Type != embed.ContentText {
		return embed.Result{}, fmt.Errorf("openai embed: only text content is supported")
	}

	text, ok := req.Content.(string)
	if !ok || text == "" {
		return embed.Result{}, fmt.Errorf("openai embed: empty text content")
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	var result embed.Result
	err := embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, text)
		return rerr
	}, 3, time.Second)

	return result, err
}

// EmbedBatch embeds multiple texts in a single API call.
func (e *Engine) EmbedBatch(ctx context.Context, reqs []embed.Request) ([]embed.Result, error) {
	texts := make([]string, len(reqs))
	for i, req := range reqs {
		if req.Type != embed.ContentText {
			return nil, fmt.Errorf("openai embed batch: item %d is not text", i)
		}
		text, ok := req.Content.(string)
		if !ok || text == "" {
			return nil, fmt.Errorf("openai embed batch: item %d is empty", i)
		}
		texts[i] = text
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	var results []embed.Result
	err := embed.Retry(func() error {
		var rerr error
		results, rerr = e.doBatchEmbed(ctx, texts)
		return rerr
	}, 3, time.Second)

	return results, err
}

// EmbedCapabilities implements embed.Describer.
func (e *Engine) EmbedCapabilities() embed.Capability {
	return embed.Capability{
		SupportedTypes: []embed.ContentType{embed.ContentText},
		Models:         []string{"text-embedding-3-small", "text-embedding-3-large", "text-embedding-ada-002"},
		MaxDimensions:  3072,
		SupportsMRL:    true,
	}
}

func (e *Engine) doEmbed(ctx context.Context, text string) (embed.Result, error) {
	apiReq := apiRequest{
		Model:      e.model,
		Input:      text,
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
		return embed.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "openai-embed")
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return embed.Result{}, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Data) == 0 || len(apiResp.Data[0].Embedding) == 0 {
		return embed.Result{}, fmt.Errorf("openai embed: empty embedding returned")
	}

	return embed.Result{
		Vector:     apiResp.Data[0].Embedding,
		Dimensions: len(apiResp.Data[0].Embedding),
		Model:      apiResp.Model,
	}, nil
}

func (e *Engine) doBatchEmbed(ctx context.Context, texts []string) ([]embed.Result, error) {
	apiReq := apiBatchRequest{
		Model:      e.model,
		Input:      texts,
		Dimensions: e.dimensions,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, aigoerr.FromHTTPResponse(resp, respBody, "openai-embed")
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]embed.Result, len(apiResp.Data))
	for i, d := range apiResp.Data {
		results[i] = embed.Result{
			Vector:     d.Embedding,
			Dimensions: len(d.Embedding),
			Model:      apiResp.Model,
		}
	}
	return results, nil
}

// --- API types ---

type apiRequest struct {
	Model      string `json:"model"`
	Input      string `json:"input"`
	Dimensions int    `json:"dimensions,omitempty"`
}

type apiBatchRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type apiResponse struct {
	Data  []apiEmbeddingData `json:"data"`
	Model string             `json:"model"`
}

type apiEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
