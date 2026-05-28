package alibabacloud

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/alibabacloud/internal/audiogen"
	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/imggen"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/engine/alibabacloud/internal/threedgen"
	"github.com/godeps/aigo/engine/alibabacloud/internal/vidgen"
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

	// Tripo 专业 3D 生成模型（最高 2 万面）。
	ModelTripoP1 = "Tripo/Tripo-P1.0"
	// Tripo 高精度 3D 生成模型（最高 200 万面，支持 geometry_quality）。
	ModelTripoH31 = "Tripo/Tripo-H3.1"

	ModelHappyHorseT2V       = "happyhorse-1.0-t2v"
	ModelHappyHorseI2V       = "happyhorse-1.0-i2v"
	ModelHappyHorseR2V       = "happyhorse-1.0-r2v"
	ModelHappyHorseVideoEdit = "happyhorse-1.0-video-edit"

	ModelFunMusic = "fun-music-v1"
)

// modelAliases maps routing-suffixed model names to their canonical
// DashScope model name. The suffix is a saker-level routing convention
// (e.g. "-i2i" for image-to-image); the canonical name is what the
// DashScope API actually expects.
var modelAliases = map[string]string{
	// Image: -t2i / -i2i → same multimodal handler
	"qwen-image-t2i":     ModelQwenImage,
	"qwen-image-i2i":     ModelQwenImage,
	"qwen-image-2.0-t2i": ModelQwenImage2,
	"qwen-image-2.0-i2i": ModelQwenImage2,
	"wan2.7-image-t2i":   ModelWanImage,
	"wan2.7-image-i2i":   ModelWanImage,
	"z-image-turbo-t2i":  ModelZImageTurbo,
	"z-image-turbo-i2i":  ModelZImageTurbo,

	// Video edit: -style → same RunVideoEdit handler
	"wan2.7-style":          ModelWanVideoEdit,
	"happyhorse-1.0-style":  ModelHappyHorseVideoEdit,

	// 3D: -t23d / -i23d / -mv23d → same RunTripo3D handler
	"Tripo/Tripo-P1.0-t23d":  ModelTripoP1,
	"Tripo/Tripo-P1.0-i23d":  ModelTripoP1,
	"Tripo/Tripo-P1.0-mv23d": ModelTripoP1,
	"Tripo/Tripo-H3.1-t23d":  ModelTripoH31,
	"Tripo/Tripo-H3.1-i23d":  ModelTripoH31,
	"Tripo/Tripo-H3.1-mv23d": ModelTripoH31,
}

// ResolveModel maps a routing-suffixed model name to its canonical
// DashScope model name. Returns the input unchanged if not an alias.
func ResolveModel(model string) string {
	if canonical, ok := modelAliases[model]; ok {
		return canonical
	}
	return model
}

// 与 internal/ierr 中哨兵为同一指针，便于 errors.Is。
var (
	ErrMissingPrompt      = ierr.ErrMissingPrompt
	ErrMissingReference   = ierr.ErrMissingReference
	ErrMissingVoice       = ierr.ErrMissingVoice
	ErrMissingVoiceDesign = ierr.ErrMissingVoiceDesign
	ErrMissingAudioURL    = ierr.ErrMissingAudioURL
	ErrUnsupportedModel   = ierr.ErrUnsupportedModel

	// Tripo 3D 边界错误。
	ErrMissingTripoInput  = ierr.ErrMissingTripoInput
	ErrTooManyTripoImages = ierr.ErrTooManyTripoImages
	ErrTripoPromptTooLong = ierr.ErrTripoPromptTooLong

	// HappyHorse 边界错误。
	ErrTooManyHappyHorseImages          = ierr.ErrTooManyHappyHorseImages
	ErrHappyHorseVideoEditMissingVideo  = ierr.ErrHappyHorseVideoEditMissingVideo
	ErrHappyHorseVideoEditTooManyImages = ierr.ErrHappyHorseVideoEditTooManyImages

	// Wan video-edit 边界错误。
	ErrWanVideoEditMissingVideo  = ierr.ErrWanVideoEditMissingVideo
	ErrWanVideoEditTooManyImages = ierr.ErrWanVideoEditTooManyImages
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
	model = ResolveModel(model)

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

// aliyunMultiHandler is an opt-in extension for handlers that can produce
// multiple result items in a single call (e.g. multimodal-image with n>1).
// The first return value remains the primary URL for backward compatibility;
// the second carries the full result set when len > 1.
type aliyunMultiHandler func(ctx context.Context, rt *runtime.RT, apiKey, model string, graph workflow.Graph) (string, []engine.ResultItem, error)

type modelEntry struct {
	handler      aliyunHandler      // single-result handler (mutually exclusive with multiHandler)
	multiHandler aliyunMultiHandler // optional multi-result handler
	kind         engine.OutputKind
}

var modelTable = map[string]modelEntry{
	ModelQwenImage:            {handler: imggen.RunQwenImage, kind: engine.OutputURL},
	ModelQwenImage2:           {multiHandler: imggen.RunMultimodalImageMulti, kind: engine.OutputURL},
	ModelQwenImageEditPlus:    {multiHandler: imggen.RunMultimodalImageMulti, kind: engine.OutputURL},
	ModelWanImage:             {multiHandler: imggen.RunMultimodalImageMulti, kind: engine.OutputURL},
	ModelZImageTurbo:          {multiHandler: imggen.RunMultimodalImageMulti, kind: engine.OutputURL},
	ModelWanTextToVideo:       {handler: vidgen.RunTextToVideo, kind: engine.OutputURL},
	ModelWanImageToVideo:      {handler: vidgen.RunReferenceToVideo, kind: engine.OutputURL},
	ModelWanReferenceVideo:    {handler: vidgen.RunReferenceToVideo, kind: engine.OutputURL},
	ModelWanVideoEdit:         {handler: vidgen.RunVideoEdit, kind: engine.OutputURL},
	ModelKlingV3Video:         {handler: vidgen.RunKlingVideo, kind: engine.OutputURL},
	ModelKlingV3OmniVideo:     {handler: vidgen.RunKlingVideo, kind: engine.OutputURL},
	ModelQwenTTSFlash:         {handler: audiogen.RunTTS, kind: engine.OutputURL},
	ModelQwenTTSInstructFlash: {handler: audiogen.RunTTS, kind: engine.OutputURL},
	ModelQwenVoiceDesign:      {handler: audiogen.RunVoiceDesign, kind: engine.OutputJSON},
	ModelQwenASRFlash:          {handler: audiogen.RunQwenASR, kind: engine.OutputPlainText},
	ModelQwenASRFlashFiletrans: {handler: audiogen.RunQwenASRFiletrans, kind: engine.OutputPlainText},
	ModelTripoP1:               {handler: threedgen.RunTripo3D, kind: engine.OutputURL},
	ModelTripoH31:              {handler: threedgen.RunTripo3D, kind: engine.OutputURL},
	ModelHappyHorseT2V:         {handler: vidgen.RunHappyHorseTextToVideo, kind: engine.OutputURL},
	ModelHappyHorseI2V:         {handler: vidgen.RunHappyHorseImageToVideo, kind: engine.OutputURL},
	ModelHappyHorseR2V:         {handler: vidgen.RunHappyHorseReferenceToVideo, kind: engine.OutputURL},
	ModelHappyHorseVideoEdit:   {handler: vidgen.RunHappyHorseVideoEdit, kind: engine.OutputURL},
	ModelFunMusic:              {handler: audiogen.RunMusic, kind: engine.OutputURL},
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

	var (
		value   string
		results []engine.ResultItem
	)
	switch {
	case entry.multiHandler != nil:
		value, results, err = entry.multiHandler(ctx, &e.rt, apiKey, e.model, graph)
	case entry.handler != nil:
		value, err = entry.handler(ctx, &e.rt, apiKey, e.model, graph)
	default:
		return engine.Result{}, fmt.Errorf("aliyun: model %q has no handler registered", e.model)
	}
	if err != nil {
		return engine.Result{}, err
	}
	kind := entry.kind
	if kind == engine.OutputURL && strings.HasPrefix(value, "data:") {
		kind = engine.OutputDataURI
	}
	out := engine.Result{Value: value, Kind: kind}
	// Only expose Results when there's genuinely more than one item — single-result
	// callers should keep using Value alone for backward compatibility.
	if len(results) > 1 {
		out.Results = results
	}
	return out, nil
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
	case ModelHappyHorseT2V:
		cap.MediaTypes = []string{"video"}
		cap.Sizes = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "4:5", "5:4"}
		cap.MaxDuration = 15
		cap.MaxPromptChars = 2500
	case ModelHappyHorseI2V:
		cap.MediaTypes = []string{"video"}
		cap.MaxImages = 1
		cap.MaxDuration = 15
		cap.MaxPromptChars = 2500
	case ModelHappyHorseR2V:
		cap.MediaTypes = []string{"video"}
		cap.Sizes = []string{"16:9", "9:16", "1:1", "4:3", "3:4", "4:5", "5:4"}
		cap.MaxImages = 9
		cap.MaxDuration = 15
		cap.MaxPromptChars = 2500
	case ModelHappyHorseVideoEdit:
		cap.MediaTypes = []string{"video"}
		cap.MaxImages = 5
		cap.MaxDuration = 15
		cap.MaxPromptChars = 2500
	case ModelQwenTTSFlash, ModelQwenTTSInstructFlash:
		cap.MediaTypes = []string{"audio"}
		cap.Voices = []string{"Cherry", "Serena", "Ethan", "Chelsie"}
	case ModelQwenVoiceDesign:
		cap.MediaTypes = []string{"audio"}
	case ModelQwenASRFlash, ModelQwenASRFlashFiletrans:
		cap.MediaTypes = []string{"audio"}
	case ModelTripoP1, ModelTripoH31:
		cap.MediaTypes = []string{"3d"}
		cap.MaxImages = 4         // 多图生 3D 上限。
		cap.MaxPromptChars = 1024 // Tripo prompt 字符上限。
	case ModelFunMusic:
		cap.MediaTypes = []string{"audio"}
		cap.MaxPromptChars = 2000
	}
	return cap
}

// editModels lists models that are editors, not generators.
// Used by ModelsByCapability to classify them under "*_edit" keys.
var editModels = map[string]string{
	ModelQwenImageEditPlus:   "image_edit",
	ModelWanVideoEdit:        "video_edit",
	ModelHappyHorseVideoEdit: "video_edit",
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

// musicModels lists models that are music generation, not TTS.
// Used by ModelsByCapability to classify them under "music" key.
var musicModels = map[string]bool{
	ModelFunMusic: true,
}

// ConfigSchema returns the configuration fields required by the Aliyun engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "apiKey", Label: "API Key", Type: "secret", Required: true, EnvVar: "DASHSCOPE_API_KEY", Description: "DashScope API key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "DASHSCOPE_BASE_URL", Description: "Custom API base URL (optional)"},
	}
}

// modelSortPriority returns 0 for sync models (multiHandler) and 1 for async
// models (handler). Lower priority sorts first in the routing slice.
func modelSortPriority(model string) int {
	if entry, ok := modelTable[model]; ok && entry.multiHandler != nil {
		return 0
	}
	return 1
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
			} else if musicModels[model] {
				key = "music"
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
	// Sort each capability slice: sync models (multiHandler) before async
	// (handler), then alphabetically for deterministic order across restarts.
	for _, models := range result {
		slices.SortFunc(models, func(a, b string) int {
			pa, pb := modelSortPriority(a), modelSortPriority(b)
			if pa != pb {
				return cmp.Compare(pa, pb)
			}
			return cmp.Compare(a, b)
		})
	}
	return result
}
