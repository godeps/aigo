package newapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/graph"
	"github.com/godeps/aigo/engine/newapi/internal/rt"
	"github.com/godeps/aigo/workflow"
)

// MediaKind 在未指定 Route 时选择默认 OpenAI 兼容族。
type MediaKind string

const (
	KindImage  MediaKind = "image"
	KindVideo  MediaKind = "video"
	KindSpeech MediaKind = "speech"
)

const (
	defaultPollInterval = 5 * time.Second
)

// Config 配置 New API 引擎。
type Config struct {
	APIKey  string
	BaseURL string // 网关 origin 或以 /v1 结尾的写法均可，见 NormalizeOrigin
	Model   string
	// Route 非空时优先；否则按 Kind 选择默认 OpenAI 路径族。
	Route Route
	Kind  MediaKind

	HTTPClient *http.Client
	// 图像
	Quality string
	Style   string
	// 视频
	WaitForCompletion bool
	PollInterval      time.Duration
	// 即梦
	JimengVersion string // 查询参数 Version，默认 2022-08-31
}

// Engine 实现 engine.Engine。
type Engine struct {
	origin       string
	route        Route
	kind         MediaKind
	model        string
	apiKey       string
	quality      string
	style        string
	httpClient   *http.Client
	waitVideo    bool
	pollInterval time.Duration
	jimengVer    string
}

// New 创建引擎。Kind 为空且 Route 为空时，默认 KindImage。
func New(cfg Config) *Engine {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	kind := cfg.Kind
	if kind == "" && cfg.Route == RouteAuto {
		kind = KindImage
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("NEWAPI_BASE_URL")), "/")
	}
	origin := rt.NormalizeOrigin(base)

	jv := strings.TrimSpace(cfg.JimengVersion)
	if jv == "" {
		jv = "2022-08-31"
	}

	return &Engine{
		origin:       origin,
		route:        cfg.Route,
		kind:         kind,
		model:        strings.TrimSpace(cfg.Model),
		apiKey:       strings.TrimSpace(cfg.APIKey),
		quality:      cfg.Quality,
		style:        cfg.Style,
		httpClient:   hc,
		waitVideo:    cfg.WaitForCompletion,
		pollInterval: poll,
		jimengVer:    jv,
	}
}

func (e *Engine) effectiveRoute() Route {
	if e.route != RouteAuto && e.route != "" {
		return e.route
	}
	return defaultRouteForKind(e.kind)
}

func (e *Engine) apiURL(path string) string {
	return rt.Join(e.origin, path)
}

// Execute 按 Route/Kind 调用对应 HTTP 接口。
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (string, error) {
	if err := g.Validate(); err != nil {
		return "", fmt.Errorf("newapi: validate graph: %w", err)
	}
	if e.origin == "" {
		return "", ErrMissingBaseURL
	}
	if e.model == "" {
		return "", fmt.Errorf("newapi: Model is empty")
	}

	apiKey := e.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("NEWAPI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("NEWAPI_KEY")
	}
	if apiKey == "" {
		return "", ErrMissingAPIKey
	}

	r := e.effectiveRoute()
	switch r {
	case RouteOpenAIImagesGenerations:
		return e.runOpenAIImageGenerations(ctx, apiKey, g)
	case RouteOpenAIImagesEdits:
		return e.runOpenAIImageEdits(ctx, apiKey, g)
	case RouteOpenAIVideoGenerations:
		return e.runOpenAIVideoGenerations(ctx, apiKey, g)
	case RouteOpenAISpeech:
		return e.runOpenAISpeech(ctx, apiKey, g)
	case RouteOpenAITranscriptions:
		return e.runOpenAIWhisper(ctx, apiKey, g, "/v1/audio/transcriptions")
	case RouteOpenAITranslations:
		return e.runOpenAIWhisper(ctx, apiKey, g, "/v1/audio/translations")
	case RouteKlingText2Video:
		return e.runKlingVideo(ctx, apiKey, g, "/kling/v1/videos/text2video")
	case RouteKlingImage2Video:
		return e.runKlingVideo(ctx, apiKey, g, "/kling/v1/videos/image2video")
	case RouteJimengVideo:
		return e.runJimengVideo(ctx, apiKey, g)
	case RouteSoraVideos:
		return e.runSoraVideo(ctx, apiKey, g)
	case RouteQwenImagesGenerations:
		return e.runQwenImageGenerations(ctx, apiKey, g)
	case RouteGeminiGenerateContent:
		return e.runGeminiGenerateContent(ctx, apiKey, g)
	default:
		return "", fmt.Errorf("newapi: unknown Route %q", r)
	}
}

func wrapGraphErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, graph.ErrMissingPrompt) {
		return ErrMissingPrompt
	}
	if errors.Is(err, graph.ErrMissingImageSource) {
		return ErrMissingImageSource
	}
	if errors.Is(err, graph.ErrMissingAudioSource) {
		return ErrMissingAudioSource
	}
	return err
}
