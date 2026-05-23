// Package gemini implements the Gemini Embedding 2 backend.
//
// Supports text, image, and native video embedding via the official Google GenAI SDK.
// Model: gemini-embedding-2-preview (768 dimensions, MRL-capable).
package gemini

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/embed"
	"google.golang.org/genai"
)

const (
	DefaultModel      = "gemini-embedding-2-preview"
	DefaultDimensions = 768
	DefaultRPM        = 55
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
	client     *genai.Client
	model      string
	dimensions int
	limiter    *embed.RateLimiter
}

// New creates a Gemini embedding engine.
func New(cfg Config) (*Engine, error) {
	apiKey, err := engine.ResolveKey(cfg.APIKey, "GEMINI_API_KEY")
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
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

	cc := &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	}
	if cfg.HTTPClient != nil {
		cc.HTTPClient = cfg.HTTPClient
	}

	client, err := genai.NewClient(context.Background(), cc)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: create client: %w", err)
	}

	return &Engine{
		client:     client,
		model:      model,
		dimensions: dims,
		limiter:    embed.NewRateLimiter(rpm),
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

	dims := int32(e.dimensions)
	embedCfg := &genai.EmbedContentConfig{
		TaskType:             taskType,
		OutputDimensionality: &dims,
	}

	var result embed.Result
	err := embed.Retry(func() error {
		resp, rerr := e.client.Models.EmbedContent(ctx, e.model, []*genai.Content{content}, embedCfg)
		if rerr != nil {
			return rerr
		}
		if resp == nil || len(resp.Embeddings) == 0 || len(resp.Embeddings[0].Values) == 0 {
			return fmt.Errorf("gemini embed: empty embedding returned")
		}
		result = embed.Result{
			Vector:     resp.Embeddings[0].Values,
			Dimensions: len(resp.Embeddings[0].Values),
			Model:      e.model,
		}
		return nil
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

func (e *Engine) buildContent(req embed.Request) *genai.Content {
	switch req.Type {
	case embed.ContentText:
		text, _ := req.Content.(string)
		return genai.NewContentFromText(text, genai.RoleUser)
	case embed.ContentImage:
		data, _ := req.Content.([]byte)
		return genai.NewContentFromBytes(data, "image/jpeg", genai.RoleUser)
	case embed.ContentVideo:
		data, _ := req.Content.([]byte)
		return genai.NewContentFromBytes(data, "video/mp4", genai.RoleUser)
	default:
		text, _ := req.Content.(string)
		return genai.NewContentFromText(text, genai.RoleUser)
	}
}
