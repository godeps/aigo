package aliyun

import (
	"context"
	"fmt"
	"net/http"
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
	ModelQwenImage2        = "qwen-image-2.0"
	ModelQwenImageEditPlus = "qwen-image-edit-plus"
	ModelWanImage          = "wan2.7-image"
	ModelZImageTurbo       = "z-image-turbo"
	ModelWanTextToVideo    = "wan2.7-t2v"
	ModelWanImageToVideo   = "wan2.7-i2v"
	ModelWanReferenceVideo = "wan2.7-r2v"
	ModelWanVideoEdit      = "wan2.7-videoedit"

	ModelKlingV3Video     = "kling/kling-v3-video-generation"
	ModelKlingV3OmniVideo = "kling/kling-v3-omni-video-generation"

	ModelQwenTTSFlash         = "qwen3-tts-flash"
	ModelQwenTTSInstructFlash = "qwen3-tts-instruct-flash"
	ModelQwenVoiceDesign      = "qwen-voice-design"

	ModelQwenASRFlash          = "qwen3-asr-flash"
	ModelQwenASRFlashFiletrans = "qwen3-asr-flash-filetrans"
)

// 与 internal/ierr 中哨兵为同一指针，便于 errors.Is。
var (
	ErrMissingPrompt      = ierr.ErrMissingPrompt
	ErrMissingReference   = ierr.ErrMissingReference
	ErrMissingVoice       = ierr.ErrMissingVoice
	ErrMissingVoiceDesign = ierr.ErrMissingVoiceDesign
	ErrMissingAudioURL    = ierr.ErrMissingAudioURL
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
	ModelQwenImage2:           {imggen.RunMultimodalImage, engine.OutputURL},
	ModelQwenImageEditPlus:    {imggen.RunMultimodalImage, engine.OutputURL},
	ModelWanImage:             {imggen.RunMultimodalImage, engine.OutputURL},
	ModelZImageTurbo:          {imggen.RunMultimodalImage, engine.OutputURL},
	ModelWanTextToVideo:       {vidgen.RunTextToVideo, engine.OutputURL},
	ModelWanImageToVideo:      {vidgen.RunReferenceToVideo, engine.OutputURL},
	ModelWanReferenceVideo:    {vidgen.RunReferenceToVideo, engine.OutputURL},
	ModelWanVideoEdit:         {vidgen.RunVideoEdit, engine.OutputURL},
	ModelKlingV3Video:         {vidgen.RunKlingVideo, engine.OutputURL},
	ModelKlingV3OmniVideo:     {vidgen.RunKlingVideo, engine.OutputURL},
	ModelQwenTTSFlash:         {audiogen.RunTTS, engine.OutputURL},
	ModelQwenTTSInstructFlash: {audiogen.RunTTS, engine.OutputURL},
	ModelQwenVoiceDesign:      {audiogen.RunVoiceDesign, engine.OutputJSON},
	ModelQwenASRFlash:          {audiogen.RunQwenASR, engine.OutputPlainText},
	ModelQwenASRFlashFiletrans: {audiogen.RunQwenASRFiletrans, engine.OutputPlainText},
}

// Execute compiles the workflow graph into the configured Bailian model request.
func (e *Engine) Execute(ctx context.Context, graph workflow.Graph) (engine.Result, error) {
	if err := graph.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("aliyun: validate graph: %w", err)
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "DASHSCOPE_API_KEY")
	if err != nil {
		return engine.Result{}, err
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
	case ModelQwenImage, ModelQwenImage2, ModelQwenImageEditPlus, ModelWanImage, ModelZImageTurbo:
		cap.MediaTypes = []string{"image"}
	case ModelWanTextToVideo, ModelWanImageToVideo, ModelWanReferenceVideo, ModelWanVideoEdit,
		ModelKlingV3Video, ModelKlingV3OmniVideo:
		cap.MediaTypes = []string{"video"}
	case ModelQwenTTSFlash, ModelQwenTTSInstructFlash:
		cap.MediaTypes = []string{"audio"}
		cap.Voices = []string{"Cherry", "Serena", "Ethan", "Chelsie"}
	case ModelQwenVoiceDesign:
		cap.MediaTypes = []string{"audio"}
	case ModelQwenASRFlash, ModelQwenASRFlashFiletrans:
		cap.MediaTypes = []string{"audio"}
	}
	return cap
}

// editModels lists models that are editors, not generators.
// Used by ModelsByCapability to classify them under "*_edit" keys.
var editModels = map[string]string{
	ModelQwenImageEditPlus: "image_edit",
	ModelWanVideoEdit:      "video_edit",
}

// dualModels lists models that support both generation and editing.
// They appear under their primary capability AND the edit capability.
var dualModels = map[string]string{
	ModelWanImage: "image_edit",
}

// asrModels lists models that are speech recognition, not synthesis.
// Used by ModelsByCapability to classify them under "asr" key.
var asrModels = map[string]bool{
	ModelQwenASRFlash:          true,
	ModelQwenASRFlashFiletrans: true,
}

// ConfigSchema returns the configuration fields required by the Aliyun engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "DASHSCOPE_API_KEY", Description: "DashScope API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "DASHSCOPE_BASE_URL", Description: "Custom API base URL (optional)"},
	}
}

// ModelsByCapability returns all supported models grouped by capability key
// (e.g. "image", "image_edit", "video", "tts"). This allows consumers to
// auto-discover models without hardcoding.
func ModelsByCapability() map[string][]string {
	result := map[string][]string{}
	for model := range modelTable {
		e := &Engine{model: model}
		cap := e.Capabilities()
		for _, mt := range cap.MediaTypes {
			key := mt
			if editKey, ok := editModels[model]; ok {
				key = editKey
			} else if asrModels[model] {
				key = "asr"
			} else if mt == "audio" && model == ModelQwenVoiceDesign {
				key = "voice_design"
			} else if mt == "audio" {
				key = "tts"
			}
			result[key] = append(result[key], model)
			// Dual models also appear under their edit capability.
			if editKey, ok := dualModels[model]; ok {
				result[editKey] = append(result[editKey], model)
			}
		}
	}
	return result
}
