package aliyun

import (
	"bytes"
	"context"
	"encoding/base64"
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
	MultimodalDefaultModel      = "multimodal-embedding-one-peace-v1"
	MultimodalDefaultDimensions = 1536
	multimodalDefaultBaseURL    = "https://dashscope.aliyuncs.com/api/v1/services/embeddings/multimodal-embedding/multimodal-embedding"
)

// MultimodalEngine implements embed.EmbedEngine for DashScope multimodal embedding.
// Supports text and image content via the multimodal-embedding-one-peace-v1 model.
type MultimodalEngine struct {
	apiKey     string
	baseURL    string
	model      string
	dimensions int
	limiter    *embed.RateLimiter
	client     *http.Client
}

// NewMultimodal creates a DashScope multimodal embedding engine.
// It reuses the same Config as the text engine (same APIKey / HTTPClient).
func NewMultimodal(cfg Config) (*MultimodalEngine, error) {
	apiKey, err := engine.ResolveKey(cfg.APIKey, "DASHSCOPE_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("aliyun multimodal embed: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = multimodalDefaultBaseURL
	}
	model := cfg.Model
	if model == "" {
		model = MultimodalDefaultModel
	}
	dims := cfg.Dimensions
	if dims <= 0 {
		dims = MultimodalDefaultDimensions
	}
	rpm := cfg.RPM
	if rpm <= 0 {
		rpm = DefaultRPM
	}

	return &MultimodalEngine{
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		dimensions: dims,
		limiter:    embed.NewRateLimiter(rpm),
		client:     httpx.OrDefault(cfg.HTTPClient, 60*time.Second),
	}, nil
}

func (e *MultimodalEngine) Dimensions() int { return e.dimensions }

// Embed produces a vector for text or image content.
func (e *MultimodalEngine) Embed(ctx context.Context, req embed.Request) (embed.Result, error) {
	if req.Type == embed.ContentVideo {
		return embed.Result{}, fmt.Errorf("aliyun multimodal embed: video content not supported, use text or image")
	}

	content, err := buildMultimodalContent(req)
	if err != nil {
		return embed.Result{}, err
	}

	if err := e.limiter.Wait(ctx); err != nil {
		return embed.Result{}, err
	}

	var result embed.Result
	err = embed.Retry(func() error {
		var rerr error
		result, rerr = e.doEmbed(ctx, content)
		return rerr
	}, 3, time.Second)

	return result, err
}

// EmbedBatch embeds multiple requests sequentially (API accepts one content item per call).
func (e *MultimodalEngine) EmbedBatch(ctx context.Context, reqs []embed.Request) ([]embed.Result, error) {
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
func (e *MultimodalEngine) EmbedCapabilities() embed.Capability {
	return embed.Capability{
		SupportedTypes: []embed.ContentType{embed.ContentText, embed.ContentImage},
		Models:         []string{MultimodalDefaultModel},
		MaxDimensions:  MultimodalDefaultDimensions,
		SupportsMRL:    false,
	}
}

func (e *MultimodalEngine) doEmbed(ctx context.Context, content mmContent) (embed.Result, error) {
	apiReq := mmRequest{
		Model: e.model,
		Input: mmInput{Contents: []mmContent{content}},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return embed.Result{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(body))
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
		return embed.Result{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return embed.Result{}, aigoerr.FromHTTPResponse(resp, respBody, "aliyun-multimodal-embed")
	}

	var apiResp mmResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return embed.Result{}, fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Output.Embeddings) == 0 {
		return embed.Result{}, fmt.Errorf("aliyun multimodal embed: empty embeddings in response")
	}

	emb := apiResp.Output.Embeddings[0]
	return embed.Result{
		Vector:     emb.Embedding,
		Dimensions: len(emb.Embedding),
		Model:      e.model,
	}, nil
}

func buildMultimodalContent(req embed.Request) (mmContent, error) {
	switch req.Type {
	case embed.ContentText:
		text, ok := req.Content.(string)
		if !ok || text == "" {
			return mmContent{}, fmt.Errorf("aliyun multimodal embed: empty text content")
		}
		return mmContent{Text: text}, nil
	case embed.ContentImage:
		data, ok := req.Content.([]byte)
		if !ok || len(data) == 0 {
			return mmContent{}, fmt.Errorf("aliyun multimodal embed: empty image content")
		}
		encoded := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
		return mmContent{Image: encoded}, nil
	default:
		return mmContent{}, fmt.Errorf("aliyun multimodal embed: unsupported content type %d", req.Type)
	}
}

// --- API types for multimodal embedding ---

type mmContent struct {
	Text  string `json:"text,omitempty"`
	Image string `json:"image,omitempty"`
}

type mmInput struct {
	Contents []mmContent `json:"contents"`
}

type mmRequest struct {
	Model string  `json:"model"`
	Input mmInput `json:"input"`
}

type mmResponse struct {
	Output mmOutput `json:"output"`
	Usage  apiUsage `json:"usage"`
}

type mmOutput struct {
	Embeddings []mmEmbedding `json:"embeddings"`
}

type mmEmbedding struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
	Type      string    `json:"type"`
}
