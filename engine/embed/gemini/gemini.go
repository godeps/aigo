// Package gemini implements the Gemini Embedding 2 backend.
//
// Supports text, image, and native video embedding via inline content parts.
// Model: gemini-embedding-2-preview (768 dimensions, MRL-capable).
package gemini

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
	DefaultModel      = "gemini-embedding-2-preview"
	DefaultDimensions = 768
	DefaultRPM        = 55
	endpointFmt       = "https://generativelanguage.googleapis.com/v1beta/models/%s:embedContent"
)

// Config configures the Gemini embedding engine.
type Config struct {
	APIKey     string
	Model      string
	Dimensions int
	RPM        int
	HTTPClient *http.Client
}

// Engine implements embed.EmbedEngine for Gemini Embedding 2.
type Engine struct {
	apiKey     string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// New creates a Gemini embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("gemini embed: GEMINI_API_KEY not set")
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
		rpm = DefaultRPM
	}

	return &Engine{
		apiKey:     apiKey,
		model:      model,
		dimensions: dims,
		limiter:    embed.NewRateLimiter(rpm),
		client:     httpx.OrDefault(cfg.HTTPClient, 120*time.Second),
	}, nil
}

func (e *Engine) Dimensions() int { return e.dimensions }

// Embed produces a vector for text, image, or video content.
func (e *Engine) Embed(ctx context.Context, req embed.Request) (embed.Result, error) {
	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	content := e.buildContent(req)
	taskType := req.TaskType
	if taskType == "" {
		taskType = "RETRIEVAL_DOCUMENT"
	}

	apiReq := apiEmbedRequest{
		Content: content,
		Config: apiEmbedConfig{
			TaskType:             taskType,
			OutputDimensionality: e.dimensions,
		},
	}

	var result embed.Result
	err := embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, apiReq)
		return rerr
	}, 5, 2*time.Second)

	return result, err
}

// EmbedBatch embeds multiple requests sequentially (Gemini has no batch API).
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
		SupportedTypes: []embed.ContentType{embed.ContentText, embed.ContentImage, embed.ContentVideo},
		Models:         []string{DefaultModel},
		MaxDimensions:  3072,
		SupportsMRL:    true,
	}
}

func (e *Engine) doEmbed(ctx context.Context, apiReq apiEmbedRequest) (embed.Result, error) {
	body, err := json.Marshal(apiReq)
	if err != nil {
		return embed.Result{}, err
	}

	url := fmt.Sprintf(endpointFmt, e.model) + "?key=" + e.apiKey
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return embed.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return embed.Result{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return embed.Result{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return embed.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "gemini-embed")
	}

	var apiResp apiEmbedResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return embed.Result{}, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Embedding.Values) == 0 {
		return embed.Result{}, fmt.Errorf("gemini embed: empty embedding returned")
	}

	return embed.Result{
		Vector:     apiResp.Embedding.Values,
		Dimensions: len(apiResp.Embedding.Values),
		Model:      e.model,
	}, nil
}

func (e *Engine) buildContent(req embed.Request) apiContent {
	switch req.Type {
	case embed.ContentText:
		text, _ := req.Content.(string)
		return apiContent{Parts: []apiPart{{Text: text}}}
	case embed.ContentImage:
		data, _ := req.Content.([]byte)
		return apiContent{Parts: []apiPart{{InlineData: &apiBlob{MimeType: "image/jpeg", Data: data}}}}
	case embed.ContentVideo:
		data, _ := req.Content.([]byte)
		return apiContent{Parts: []apiPart{{InlineData: &apiBlob{MimeType: "video/mp4", Data: data}}}}
	default:
		text, _ := req.Content.(string)
		return apiContent{Parts: []apiPart{{Text: text}}}
	}
}

// --- API types ---

type apiContent struct {
	Parts []apiPart `json:"parts"`
}

type apiPart struct {
	Text       string   `json:"text,omitempty"`
	InlineData *apiBlob `json:"inline_data,omitempty"`
}

type apiBlob struct {
	MimeType string `json:"mime_type"`
	Data     []byte `json:"data"`
}

type apiEmbedConfig struct {
	TaskType             string `json:"task_type"`
	OutputDimensionality int    `json:"output_dimensionality"`
}

type apiEmbedRequest struct {
	Content apiContent     `json:"content"`
	Config  apiEmbedConfig `json:"config"`
}

type apiEmbedResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}
