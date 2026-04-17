// Package aliyun implements the Alibaba Cloud DashScope (Bailian) embedding backend.
//
// Supports text-embedding-v3 (1024d, multilingual, MRL),
// text-embedding-v2 (1536d), and text-embedding-v1 (1536d).
// Requires DASHSCOPE_API_KEY environment variable.
package alibabacloud

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
	DefaultModel      = "text-embedding-v3"
	DefaultDimensions = 1024
	DefaultRPM        = 100
	defaultBaseURL    = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding"
)

// Config configures the DashScope embedding engine.
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Dimensions int
	RPM        int
	HTTPClient *http.Client
}

// Engine implements embed.EmbedEngine for Alibaba Cloud DashScope.
type Engine struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// New creates a DashScope embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey, err := engine.ResolveKey(cfg.APIKey, "DASHSCOPE_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("aliyun embed: %w", err)
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
		dims = defaultDimensions(model)
	}
	rpm := cfg.RPM
	if rpm <= 0 {
		rpm = DefaultRPM
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
		return embed.Result{}, fmt.Errorf("aliyun embed: only text content is supported")
	}

	text, ok := req.Content.(string)
	if !ok || text == "" {
		return embed.Result{}, fmt.Errorf("aliyun embed: empty text content")
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	textType := "document"
	if req.TaskType == "RETRIEVAL_QUERY" {
		textType = "query"
	}

	var result embed.Result
	err := embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, []string{text}, textType)
		return rerr
	}, 3, time.Second)

	return result, err
}

// EmbedBatch embeds multiple texts in a single API call.
func (e *Engine) EmbedBatch(ctx context.Context, reqs []embed.Request) ([]embed.Result, error) {
	texts := make([]string, len(reqs))
	for i, req := range reqs {
		if req.Type != embed.ContentText {
			return nil, fmt.Errorf("aliyun embed batch: item %d is not text", i)
		}
		text, ok := req.Content.(string)
		if !ok {
			return nil, fmt.Errorf("aliyun embed batch: item %d is empty", i)
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
		Models:         []string{"text-embedding-v3", "text-embedding-v2", "text-embedding-v1"},
		MaxDimensions:  1536,
		SupportsMRL:    e.model == "text-embedding-v3",
	}
}

func (e *Engine) doEmbed(ctx context.Context, texts []string, textType string) (embed.Result, error) {
	results, err := e.doBatchEmbed(ctx, texts, textType)
	if err != nil {
		return embed.Result{}, err
	}
	if len(results) == 0 {
		return embed.Result{}, fmt.Errorf("aliyun embed: empty response")
	}
	return results[0], nil
}

func (e *Engine) doBatchEmbed(ctx context.Context, texts []string, textType string) ([]embed.Result, error) {
	apiReq := apiRequest{
		Model: e.model,
		Input: apiInput{Texts: texts},
		Parameters: apiParameters{
			TextType: textType,
		},
	}
	// text-embedding-v3 supports dimension parameter
	if e.model == "text-embedding-v3" && e.dimensions > 0 {
		apiReq.Parameters.Dimension = e.dimensions
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(body))
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
		return nil, aigoerr.FromHTTPResponse(resp, respBody, "aliyun-embed")
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("aliyun embed: empty embeddings in response")
	}

	results := make([]embed.Result, len(apiResp.Output.Embeddings))
	for i, emb := range apiResp.Output.Embeddings {
		results[i] = embed.Result{
			Vector:     emb.Embedding,
			Dimensions: len(emb.Embedding),
			Model:      e.model,
		}
	}
	return results, nil
}

func defaultDimensions(model string) int {
	switch model {
	case "text-embedding-v3":
		return 1024
	case "text-embedding-v2", "text-embedding-v1":
		return 1536
	default:
		return DefaultDimensions
	}
}

// --- API types ---

type apiInput struct {
	Texts []string `json:"texts"`
}

type apiParameters struct {
	TextType  string `json:"text_type"`
	Dimension int    `json:"dimension,omitempty"`
}

type apiRequest struct {
	Model      string        `json:"model"`
	Input      apiInput      `json:"input"`
	Parameters apiParameters `json:"parameters"`
}

type apiResponse struct {
	Output apiOutput `json:"output"`
	Usage  apiUsage  `json:"usage"`
}

type apiOutput struct {
	Embeddings []apiEmbedding `json:"embeddings"`
}

type apiEmbedding struct {
	TextIndex int       `json:"text_index"`
	Embedding []float32 `json:"embedding"`
}

type apiUsage struct {
	TotalTokens int `json:"total_tokens"`
}
