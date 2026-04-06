package aliyun

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aliyun/internal/audiogen"
	"github.com/godeps/aigo/engine/aliyun/internal/ierr"
	"github.com/godeps/aigo/engine/aliyun/internal/imggen"
	"github.com/godeps/aigo/engine/aliyun/internal/runtime"
	"github.com/godeps/aigo/engine/aliyun/internal/vidgen"
	"github.com/godeps/aigo/engine/httpx"
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

type aliyunHandler func(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, error)

type modelEntry struct {
	handler aliyunHandler
	kind    engine.OutputKind
}

var modelTable = map[string]modelEntry{
	ModelQwenImage:            {imggen.RunQwenImage, engine.OutputURL},
	ModelWanImage:             {imggen.RunMultimodalImage, engine.OutputURL},
	ModelZImageTurbo:          {imggen.RunMultimodalImage, engine.OutputURL},
	ModelWanTextToVideo:       {vidgen.RunTextToVideo, engine.OutputURL},
	ModelWanReferenceVideo:    {vidgen.RunReferenceToVideo, engine.OutputURL},
	ModelWanVideoEdit:         {vidgen.RunVideoEdit, engine.OutputURL},
	ModelQwenTTSFlash:         {audiogen.RunTTS, engine.OutputURL},
	ModelQwenTTSInstructFlash: {audiogen.RunTTS, engine.OutputURL},
	ModelQwenVoiceDesign:      {audiogen.RunVoiceDesign, engine.OutputJSON},
}

// Execute compiles the workflow graph into the configured Bailian model request.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	if err := graph.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("aliyun: validate graph: %w", err)
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return engine.Result{}, ErrMissingAPIKey
	}

	entry, ok := modelTable[e.model]
	if !ok {
		return engine.Result{}, fmt.Errorf("%w: %s", ErrUnsupportedModel, e.model)
	}
	value, err := entry.handler(ctx, &e.rt, apiKey, e.model, graph)
	if err != nil {
		return engine.Result{}, err
	}
	kind := entry.kind
	if kind == engine.OutputURL && strings.HasPrefix(value, "data:") {
		kind = engine.OutputDataURI
	}
	return engine.Result{Value: value, Kind: kind}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	cap := engine.Capability{
		Models:       []string{e.model},
		SupportsPoll: e.rt.WaitForCompletion,
		SupportsSync: !e.rt.WaitForCompletion,
	}
	switch e.model {
	case ModelQwenImage, ModelWanImage, ModelZImageTurbo:
		cap.MediaTypes = []string{"image"}
	case ModelWanTextToVideo, ModelWanReferenceVideo, ModelWanVideoEdit:
		cap.MediaTypes = []string{"video"}
	case ModelQwenTTSFlash, ModelQwenTTSInstructFlash:
		cap.MediaTypes = []string{"audio"}
	case ModelQwenVoiceDesign:
		cap.MediaTypes = []string{"audio"}
	}
	return cap
}
