// Package voyage implements the Voyage AI embedding backend.
//
// Supports voyage-3 (1024d), voyage-3-lite (512d), voyage-code-3 (1024d).
// Voyage is Anthropic's recommended embedding provider.
package voyage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/embed"
	"github.com/godeps/aigo/engine/httpx"
)

const (
	DefaultModel      = "voyage-3"
	DefaultDimensions = 1024
	defaultBaseURL    = "https://api.voyageai.com/v1"
)

// Config configures the Voyage embedding engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	RPM        int
	HTTPClient *http.Client
}

// Engine implements embed.EmbedEngine for Voyage AI.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// New creates a Voyage embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey, err := engine.ResolveKey(cfg.APIKey, "VOYAGE_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("voyage embed: %w", err)
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
		rpm = 300
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
		return embed.Result{}, fmt.Errorf("voyage embed: only text content is supported")
	}

	text, ok := req.Content.(string)
	if !ok || text == "" {
		return embed.Result{}, fmt.Errorf("voyage embed: empty text content")
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	inputType := "document"
	if req.TaskType == "RETRIEVAL_QUERY" {
		inputType = "query"
	}

	var result embed.Result
	err := embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, []string{text}, inputType)
		return rerr
	}, 3, time.Second)

	return result, err
}

// EmbedBatch embeds multiple texts in a single API call.
func (e *Engine) EmbedBatch(ctx context.Context, reqs []embed.Request) ([]embed.Result, error) {
	texts := make([]string, len(reqs))
	for i, req := range reqs {
		if req.Type != embed.ContentText {
			return nil, fmt.Errorf("voyage embed batch: item %d is not text", i)
		}
		text, ok := req.Content.(string)
		if !ok {
			return nil, fmt.Errorf("voyage embed batch: item %d is empty", i)
		}
		texts[i] = text
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	return e.doBatchEmbed(ctx, texts, "document")
}

// EmbedCapabilities implements embed.Describer.
func (e *Engine) EmbedCapabilities() embed.Capability {
	return embed.Capability{
		SupportedTypes: []embed.ContentType{embed.ContentText},
		Models:         []string{"voyage-3", "voyage-3-lite", "voyage-code-3"},
		MaxDimensions:  1024,
		SupportsMRL:    false,
	}
}

func (e *Engine) doEmbed(ctx context.Context, texts []string, inputType string) (embed.Result, error) {
	results, err := e.doBatchEmbed(ctx, texts, inputType)
	if err != nil {
		return embed.Result{}, err
	}
	if len(results) == 0 {
		return embed.Result{}, fmt.Errorf("voyage embed: empty response")
	}
	return results[0], nil
}

func (e *Engine) doBatchEmbed(ctx context.Context, texts []string, inputType string) ([]embed.Result, error) {
	apiReq := apiRequest{
		Model:     e.model,
		Input:     texts,
		InputType: inputType,
	}
	if e.dimensions > 0 && e.dimensions != DefaultDimensions {
		apiReq.OutputDimension = e.dimensions
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
		return nil, aigoerr.FromHTTPResponse(resp, respBody, "voyage-embed")
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
	Model           string   `json:"model"`
	Input           []string `json:"input"`
	InputType       string   `json:"input_type,omitempty"`
	OutputDimension int      `json:"output_dimension,omitempty"`
}

type apiResponse struct {
	Data  []apiEmbeddingData `json:"data"`
	Model string             `json:"model"`
}

type apiEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
