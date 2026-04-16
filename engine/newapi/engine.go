package newapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/httpx"
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
	// DisableRemoteMediaFetch 为 true 时，图中 image_url/audio_url 不再发起 HTTP GET（降低 SSRF 风险；默认 false）。
	DisableRemoteMediaFetch bool
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
	pollInterval            time.Duration
	jimengVer               string
	allowRemoteMediaFetch   bool
}

// New 创建引擎。Kind 为空且 Route 为空时，默认 KindImage。
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)
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
		origin:                origin,
		route:                 cfg.Route,
		kind:                  kind,
		model:                 strings.TrimSpace(cfg.Model),
		apiKey:                strings.TrimSpace(cfg.APIKey),
		quality:               cfg.Quality,
		style:                 cfg.Style,
		httpClient:            hc,
		waitVideo:             cfg.WaitForCompletion,
		pollInterval:          poll,
		jimengVer:             jv,
		allowRemoteMediaFetch: !cfg.DisableRemoteMediaFetch,
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
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("newapi: validate graph: %w", err)
	}
	if e.origin == "" {
		return engine.Result{}, ErrMissingBaseURL
	}
	if e.model == "" {
		return engine.Result{}, fmt.Errorf("newapi: Model is empty")
	}

	apiKey, err := engine.ResolveKey(e.apiKey, "NEWAPI_API_KEY")
	if err != nil {
		return engine.Result{}, err
	}

	raw, err := e.dispatch(ctx, apiKey, g)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: raw, Kind: engine.ClassifyOutput(raw)}, nil
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
	if errors.Is(err, graph.ErrRemoteMediaDisabled) {
		return ErrRemoteMediaDisabled
	}
	return err
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	cap := engine.Capability{
		Models: []string{e.model},
	}
	switch e.kind {
	case KindImage:
		cap.MediaTypes = []string{"image"}
		cap.SupportsSync = true
	case KindVideo:
		cap.MediaTypes = []string{"video"}
		cap.SupportsPoll = e.waitVideo
		cap.SupportsSync = !e.waitVideo
	case KindSpeech:
		cap.MediaTypes = []string{"audio"}
		cap.SupportsSync = true
	default:
		cap.MediaTypes = []string{"image"}
		cap.SupportsSync = true
	}
	return cap
}
