package aliyun

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/aliyun/internal/audiogen"
	"github.com/godeps/aigo/engine/httpx"
	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
	"github.com/godeps/aigo/engine/aliyun/internal/imggen"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/engine/aliyun/internal/vidgen"
	"github.com/godeps/aigo/workflow"
)

const (
	defaultBaseURL = "https://dashscope.aliyuncs.com/api/v1"

	ModelQwenImage         = "qwen-image"
	ModelWanImage          = "wan2.7-image"
	ModelZImageTurbo       = "z-image-turbo"
	ModelWanTextToVideo    = "wan2.6-t2v"
	ModelWanReferenceVideo = "wan2.6-r2v"
	ModelWanVideoEdit      = "wan2.7-videoedit"

	ModelQwenTTSFlash         = "qwen3-tts-flash"
	ModelQwenTTSInstructFlash = "qwen3-tts-instruct-flash"
	ModelQwenVoiceDesign      = "qwen-voice-design"
)

// 与 internal/ierr 中哨兵为同一指针，便于 errors.Is。
var (
	ErrMissingPrompt      = ierr.ErrMissingPrompt
	ErrMissingReference   = ierr.ErrMissingReference
	ErrMissingVoice       = ierr.ErrMissingVoice
	ErrMissingVoiceDesign = ierr.ErrMissingVoiceDesign
	ErrMissingAPIKey      = ierr.ErrMissingAPIKey
	ErrUnsupportedModel   = ierr.ErrUnsupportedModel
)

// Config configures the Alibaba Cloud Bailian engine.
type Config struct {
	APIKey            string
	BaseURL           string
	Model             string
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration
}

// Engine compiles a workflow graph into a Bailian backend request.
type Engine struct {
	rt     runtime.RT
	model  string
	apiKey string
}

// New creates a Bailian execution engine.
func New(cfg Config) *Engine {
	httpClient := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = ModelQwenImage
	}

	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = runtime.DefaultPollInterval
	}

	return &Engine{
		rt: runtime.RT{
			BaseURL:           baseURL,
			HTTPClient:        httpClient,
			WaitForCompletion: cfg.WaitForCompletion,
			PollInterval:      pollInterval,
		},
		model:  model,
		apiKey: strings.TrimSpace(cfg.APIKey),
	}
}

// Execute compiles the workflow graph into the configured Bailian model request.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (string, error) {
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("aliyun: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return "", ErrMissingAPIKey
	}

	switch {
	case imggen.IsQwenImageModel(e.model):
		return imggen.RunQwenImage(ctx, &e.rt, apiKey, e.model, graph)
	case imggen.IsMultimodalImageModel(e.model):
		return imggen.RunMultimodalImage(ctx, &e.rt, apiKey, e.model, graph)
	case isQwenVoiceDesignModel(e.model):
		return audiogen.RunVoiceDesign(ctx, &e.rt, apiKey, e.model, graph)
	case audiogen.IsTTSModel(e.model):
		return audiogen.RunTTS(ctx, &e.rt, apiKey, e.model, graph)
	case vidgen.IsVideoEditModel(e.model):
		return vidgen.RunVideoEdit(ctx, &e.rt, apiKey, e.model, graph)
	case vidgen.IsReferenceToVideoModel(e.model):
		return vidgen.RunReferenceToVideo(ctx, &e.rt, apiKey, e.model, graph)
	case vidgen.IsTextToVideoModel(e.model):
		return vidgen.RunTextToVideo(ctx, &e.rt, apiKey, e.model, graph)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedModel, e.model)
	}
}

func isQwenVoiceDesignModel(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), ModelQwenVoiceDesign)
}
