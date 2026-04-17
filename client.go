package aigo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
)

var (
	ErrEngineNil       = errors.New("aigo: engine is nil")
	ErrEngineExists    = errors.New("aigo: engine already registered")
	ErrEngineNotFound  = errors.New("aigo: engine not found")
	ErrEngineNameEmpty = errors.New("aigo: engine name is empty")
	ErrEngineDisabled  = errors.New("aigo: engine is disabled")
)

// ReferenceType identifies the kind of remote asset to attach to an agent task.
type ReferenceType string

const (
	ReferenceTypeImage ReferenceType = "image"
	ReferenceTypeVideo ReferenceType = "video"
)

// ReferenceAsset describes an externally reachable media input.
type ReferenceAsset struct {
	Type ReferenceType
	URL  string
}

// TTSOptions groups text-to-speech parameters.
type TTSOptions struct {
	Voice                string
	LanguageType         string
	Instructions         string
	OptimizeInstructions *bool
}

// MusicOptions groups music generation parameters.
type MusicOptions struct {
	Lyrics         string
	IsInstrumental *bool
	LyricsOptimizer *bool
	OutputFormat   string // "url" or "hex"
	SampleRate     int
	Bitrate        int
	Format         string // "mp3", "wav", "flac"
}

// VoiceDesignOptions groups voice design parameters.
type VoiceDesignOptions struct {
	VoicePrompt    string
	PreviewText    string
	TargetModel    string
	PreferredName  string
	Language       string
	SampleRate     int
	ResponseFormat string
	OmitPreview    bool
}

// Result is the public outcome of every Client execution method.
type Result struct {
	Value    string         // Raw output from the engine.
	Kind     OutputKind     // Authoritative classification from the engine.
	Engine   string         // Name of the engine that produced the result.
	Elapsed  time.Duration  // Wall-clock execution time.
	Metadata map[string]any // Engine-specific data (optional).
}

// String returns the raw output value, allowing fmt.Sprint(result) to work naturally.
func (r Result) String() string { return r.Value }

// AgentTask is a graph-free request shape for agents.
type AgentTask struct {
	Prompt         string
	NegativePrompt string
	Width          int
	Height         int
	Size           string
	Duration       int
	Watermark      *bool
	References     []ReferenceAsset

	TTS         *TTSOptions
	VoiceDesign *VoiceDesignOptions
	Music       *MusicOptions

	// Structured groups image/video options separately for finer control.
	Structured *AgentTaskStructured
}

// AgentTaskStructured 将图像与视频选项分组；便于扩展而无需继续增大 AgentTask 扁平字段。
type AgentTaskStructured struct {
	ImageSize      string
	ImageWatermark *bool
	VideoDuration  int
	VideoSize      string
	VideoWatermark *bool
	VideoAspectRatio string
	VideoResolution  string // "480P", "720P", "1080P"
	VideoAudio       *bool
}

// Selection is the selector's routing decision.
type Selection struct {
	Engine string
	Reason string
}

// RoutedResult is the result of a selector-driven execution.
type RoutedResult struct {
	Engine string
	Reason string
	Output Result
}

// Selector decides which registered engine should handle a task.
type Selector interface {
	SelectEngine(ctx context.Context, task AgentTask, engines []string) (Selection, error)
}

// EngineInfo describes a candidate engine's capabilities, provided to RichSelector for informed decisions.
type EngineInfo struct {
	Name        string             `json:"name"`
	DisplayName engine.DisplayName `json:"display_name"`
	Capability  engine.Capability  `json:"capability"`
}

// RichSelector is an enhanced selector that receives engine capability metadata.
// Implementations can use capability information to make better routing decisions.
// A RichSelector is also a Selector — ExecuteTaskAuto auto-detects the interface.
type RichSelector interface {
	Selector
	SelectEngineFromCandidates(ctx context.Context, task AgentTask, candidates []EngineInfo) (Selection, error)
}

// Middleware wraps an engine to add cross-cutting behavior (logging, timing, retry, etc.).
type Middleware func(name string, next engine.Engine) engine.Engine

// Client routes a workflow graph to a registered execution engine.
type Client struct {
	mu         sync.RWMutex
	engines    map[string]engine.Engine
	disabled   map[string]bool // engines that are registered but temporarily disabled
	middleware []Middleware
	store      TaskStore
}

// ClientOption configures the Client at construction time.
type ClientOption func(*Client)

// WithStore attaches a TaskStore for async task persistence and crash recovery.
func WithStore(s TaskStore) ClientOption {
	return func(c *Client) { c.store = s }
}

// WithDefaultStore attaches a FileTaskStore at .aigo/tasks.json (current working directory).
// Returns an error if the store directory cannot be created.
func WithDefaultStore() (ClientOption, error) {
	s, err := DefaultFileTaskStore()
	if err != nil {
		return nil, err
	}
	return WithStore(s), nil
}

// NewClient creates a new SDK client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		engines: make(map[string]engine.Engine),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// RegisterEngine registers an engine under a logical name.
func (c *Client) RegisterEngine(name string, e engine.Engine) error {
	if name == "" {
		return ErrEngineNameEmpty
	}
	if e == nil {
		return ErrEngineNil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.engines[name]; exists {
		return fmt.Errorf("%w: %s", ErrEngineExists, name)
	}

	c.engines[name] = e
	return nil
}

// UnregisterEngine removes a previously registered engine.
// Returns ErrEngineNotFound if the name is not registered.
func (c *Client) UnregisterEngine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.engines[name]; !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, name)
	}
	delete(c.engines, name)
	delete(c.disabled, name)
	return nil
}

// DisableEngine temporarily disables a registered engine without removing it.
// Disabled engines are excluded from Execute, EngineNames, EngineInfos, and AvailableFor.
// Returns ErrEngineNotFound if the name is not registered.
func (c *Client) DisableEngine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.engines[name]; !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, name)
	}
	if c.disabled == nil {
		c.disabled = make(map[string]bool)
	}
	c.disabled[name] = true
	return nil
}

// EnableEngine re-enables a previously disabled engine.
// Returns ErrEngineNotFound if the name is not registered.
func (c *Client) EnableEngine(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.engines[name]; !ok {
		return fmt.Errorf("%w: %s", ErrEngineNotFound, name)
	}
	delete(c.disabled, name)
	return nil
}

// IsEnabled reports whether a registered engine is currently enabled.
func (c *Client) IsEnabled(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, registered := c.engines[name]
	return registered && !c.disabled[name]
}

// RegisterEngineIfKey registers an engine only if the required API key is available.
// It checks the environment variables in order; if any is set, the engine is registered.
// Returns true if the engine was registered, false if skipped (no key found).
func (c *Client) RegisterEngineIfKey(name string, e engine.Engine, envVars ...string) (bool, error) {
	for _, env := range envVars {
		if os.Getenv(env) != "" {
			return true, c.RegisterEngine(name, e)
		}
	}
	return false, nil
}

// EngineEntry describes an engine to register with optional env-var gating.
type EngineEntry struct {
	Name    string
	Engine  engine.Engine
	EnvVars []string // required env vars; if empty, always register
}

// RegisterAll registers multiple engines at once.
// Stops and returns the first error encountered.
func (c *Client) RegisterAll(engines map[string]engine.Engine) error {
	for name, e := range engines {
		if err := c.RegisterEngine(name, e); err != nil {
			return err
		}
	}
	return nil
}

// RegisterAllIfKey registers multiple engines, each gated by env vars.
// Engines whose env vars are not set are silently skipped.
// Returns the names of engines that were actually registered.
func (c *Client) RegisterAllIfKey(entries []EngineEntry) ([]string, error) {
	var registered []string
	for _, e := range entries {
		if len(e.EnvVars) == 0 {
			if err := c.RegisterEngine(e.Name, e.Engine); err != nil {
				return registered, err
			}
			registered = append(registered, e.Name)
			continue
		}
		ok, err := c.RegisterEngineIfKey(e.Name, e.Engine, e.EnvVars...)
		if err != nil {
			return registered, err
		}
		if ok {
			registered = append(registered, e.Name)
		}
	}
	return registered, nil
}

// RegisterProvider registers all engines from a provider.
// Engines whose required env vars are not set are silently skipped.
// Returns the names of engines that were actually registered.
func (c *Client) RegisterProvider(p engine.Provider) ([]string, error) {
	var registered []string
	for _, cfg := range p.Configs {
		if len(cfg.EnvVars) == 0 {
			if err := c.RegisterEngine(cfg.Name, cfg.Engine); err != nil {
				return registered, err
			}
			registered = append(registered, cfg.Name)
			continue
		}
		ok, err := c.RegisterEngineIfKey(cfg.Name, cfg.Engine, cfg.EnvVars...)
		if err != nil {
			return registered, err
		}
		if ok {
			registered = append(registered, cfg.Name)
		}
	}
	return registered, nil
}

// Use appends middleware that wraps every engine on each Execute call.
// Middleware is applied in the order added (first added = outermost wrapper).
func (c *Client) Use(mw ...Middleware) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.middleware = append(c.middleware, mw...)
}

// ProgressEvent reports execution progress to the caller.
type ProgressEvent struct {
	Phase   string        // "submitted", "polling", "completed"
	Attempt int           // poll attempt number (0 for non-polling phases)
	Elapsed time.Duration // wall-clock time since execution start
}

// ExecuteOption configures optional Execute behavior.
type ExecuteOption func(*executeConfig)

type executeConfig struct {
	onProgress func(ProgressEvent)
	middleware []Middleware
}

// WithProgress registers a callback for execution progress events.
func WithProgress(fn func(ProgressEvent)) ExecuteOption {
	return func(cfg *executeConfig) { cfg.onProgress = fn }
}

// Execute dispatches the graph to the named engine.
// Returns ErrEngineNotFound if the engine is not registered, or ErrEngineDisabled if it is disabled.
func (c *Client) Execute(ctx context.Context, engineName string, graph workflow.Graph, opts ...ExecuteOption) (Result, error) {
	c.mu.RLock()
	e, ok := c.engines[engineName]
	isDisabled := c.disabled[engineName]
	c.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrEngineNotFound, engineName)
	}
	if isDisabled {
		return Result{}, fmt.Errorf("%w: %s", ErrEngineDisabled, engineName)
	}

	if err := graph.Validate(); err != nil {
		return Result{}, fmt.Errorf("aigo: %w", err)
	}

	var cfg executeConfig
	for _, o := range opts {
		o(&cfg)
	}

	// Apply client-level middleware.
	actual := e
	c.mu.RLock()
	mws := c.middleware
	c.mu.RUnlock()
	for i := len(mws) - 1; i >= 0; i-- {
		actual = mws[i](engineName, actual)
	}

	if cfg.onProgress != nil {
		cfg.onProgress(ProgressEvent{Phase: "submitted"})
		// Inject progress callback into context so engines' internal poll.Poll
		// calls can surface polling progress without changing engine signatures.
		ctx = poll.WithOnProgress(ctx, func(attempt int, elapsed time.Duration) {
			cfg.onProgress(ProgressEvent{Phase: "polling", Attempt: attempt, Elapsed: elapsed})
		})
	}

	start := time.Now()
	er, err := actual.Execute(ctx, graph)
	elapsed := time.Since(start)
	if err != nil {
		return Result{}, fmt.Errorf("aigo: execute with engine %q: %w", engineName, err)
	}

	kind := er.Kind
	if kind == engine.OutputUnknown {
		kind = InterpretOutputKind(er.Value)
	}

	result := Result{
		Value:   er.Value,
		Kind:    kind,
		Engine:  engineName,
		Elapsed: elapsed,
	}

	if cfg.onProgress != nil {
		cfg.onProgress(ProgressEvent{Phase: "completed", Elapsed: elapsed})
	}

	return result, nil
}

// ExecutePrompt runs the simplest agent request: a single prompt.
func (c *Client) ExecutePrompt(ctx context.Context, engineName string, prompt string) (Result, error) {
	return c.ExecuteTask(ctx, engineName, AgentTask{Prompt: prompt})
}

// ExecuteTask converts an agent task into a workflow graph and routes it to the target engine.
func (c *Client) ExecuteTask(ctx context.Context, engineName string, task AgentTask, opts ...ExecuteOption) (Result, error) {
	return c.Execute(ctx, engineName, BuildGraph(task), opts...)
}

// ExecutePromptAuto lets a selector choose the engine for a prompt-driven request.
func (c *Client) ExecutePromptAuto(ctx context.Context, selector Selector, prompt string) (RoutedResult, error) {
	return c.ExecuteTaskAuto(ctx, selector, AgentTask{Prompt: prompt})
}

// ExecuteTaskAuto lets a selector choose the engine for a structured agent task.
// If the selector implements RichSelector, capability metadata is collected and passed automatically.
func (c *Client) ExecuteTaskAuto(ctx context.Context, selector Selector, task AgentTask) (RoutedResult, error) {
	if selector == nil {
		return RoutedResult{}, errors.New("aigo: selector is nil")
	}

	var selection Selection
	var err error

	if rs, ok := selector.(RichSelector); ok {
		candidates := c.EngineInfos()
		selection, err = rs.SelectEngineFromCandidates(ctx, task, candidates)
	} else {
		engines := c.EngineNames()
		selection, err = selector.SelectEngine(ctx, task, engines)
	}
	if err != nil {
		return RoutedResult{}, fmt.Errorf("aigo: select engine: %w", err)
	}
	if selection.Engine == "" {
		return RoutedResult{}, errors.New("aigo: selector returned empty engine")
	}

	result, err := c.ExecuteTask(ctx, selection.Engine, task)
	if err != nil {
		return RoutedResult{}, err
	}

	return RoutedResult{
		Engine: selection.Engine,
		Reason: selection.Reason,
		Output: result,
	}, nil
}

// ExecuteTaskAutoWithFallback selects an engine and retries with alternatives on failure.
// If the selector is a RichSelector, all candidates receive capability metadata.
func (c *Client) ExecuteTaskAutoWithFallback(ctx context.Context, selector Selector, task AgentTask) (RoutedResult, error) {
	if selector == nil {
		return RoutedResult{}, errors.New("aigo: selector is nil")
	}

	candidates := c.EngineInfos()
	graph := BuildGraph(task)

	// Collect a ranked list of engines from the selector.
	var ranked []string
	if rs, ok := selector.(RichSelector); ok {
		sel, err := rs.SelectEngineFromCandidates(ctx, task, candidates)
		if err != nil {
			return RoutedResult{}, fmt.Errorf("aigo: select engine: %w", err)
		}
		ranked = append(ranked, sel.Engine)
		// Add remaining candidates as fallbacks.
		for _, c := range candidates {
			if c.Name != sel.Engine {
				ranked = append(ranked, c.Name)
			}
		}
	} else {
		engines := c.EngineNames()
		sel, err := selector.SelectEngine(ctx, task, engines)
		if err != nil {
			return RoutedResult{}, fmt.Errorf("aigo: select engine: %w", err)
		}
		ranked = append(ranked, sel.Engine)
		for _, name := range engines {
			if name != sel.Engine {
				ranked = append(ranked, name)
			}
		}
	}

	fr, err := c.ExecuteWithFallback(ctx, ranked, graph)
	if err != nil {
		return RoutedResult{}, err
	}
	return RoutedResult{
		Engine: fr.Engine,
		Output: fr.Output,
		Reason: fmt.Sprintf("selected with fallback (skipped %d)", len(fr.Skipped)),
	}, nil
}

// EngineNames returns enabled engine names in deterministic order.
// Disabled engines are excluded.
func (c *Client) EngineNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.engines))
	for name := range c.engines {
		if !c.disabled[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// EngineInfos returns capability metadata for all enabled engines, sorted by name.
// Disabled engines are excluded. Engines that do not implement Describer get an empty Capability.
func (c *Client) EngineInfos() []EngineInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos := make([]EngineInfo, 0, len(c.engines))
	for name, e := range c.engines {
		if c.disabled[name] {
			continue
		}
		info := EngineInfo{Name: name}
		if d, ok := e.(engine.Describer); ok {
			info.Capability = d.Capabilities()
		}
		if n, ok := e.(engine.Namer); ok {
			info.DisplayName = n.DisplayName()
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// BuildGraph compiles a high-level agent task into the SDK's workflow graph format.
func BuildGraph(task AgentTask) workflow.Graph {
	graph := workflow.Graph{
		"1": {
			ClassType: "CLIPTextEncode",
			Inputs: map[string]any{
				"text": task.Prompt,
			},
		},
	}

	nextID := 2

	if task.Width > 0 || task.Height > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "EmptyLatentImage",
			Inputs: map[string]any{
				"width":  task.Width,
				"height": task.Height,
			},
		}
		nextID++
	}

	if task.NegativePrompt != "" {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "NegativePrompt",
			Inputs: map[string]any{
				"negative_prompt": task.NegativePrompt,
			},
		}
		nextID++
	}

	imgSize := task.Size
	imgWM := task.Watermark
	vidDur := task.Duration
	vidSize := task.Size
	vidWM := task.Watermark
	var vidAspectRatio string
	var vidResolution string
	var vidAudio *bool
	if task.Structured != nil {
		if task.Structured.ImageSize != "" {
			imgSize = task.Structured.ImageSize
		}
		if task.Structured.ImageWatermark != nil {
			imgWM = task.Structured.ImageWatermark
		}
		if task.Structured.VideoDuration > 0 {
			vidDur = task.Structured.VideoDuration
		}
		if task.Structured.VideoSize != "" {
			vidSize = task.Structured.VideoSize
		}
		if task.Structured.VideoWatermark != nil {
			vidWM = task.Structured.VideoWatermark
		}
		if task.Structured.VideoAspectRatio != "" {
			vidAspectRatio = task.Structured.VideoAspectRatio
		}
		if task.Structured.VideoResolution != "" {
			vidResolution = task.Structured.VideoResolution
		}
		if task.Structured.VideoAudio != nil {
			vidAudio = task.Structured.VideoAudio
		}
	}

	imageOptions := map[string]any{}
	videoOptions := map[string]any{}

	if imgSize != "" {
		imageOptions["size"] = imgSize
	}
	if vidSize != "" {
		videoOptions["size"] = vidSize
	}
	if vidDur > 0 {
		videoOptions["duration"] = vidDur
	}
	if vidAspectRatio != "" {
		videoOptions["aspect_ratio"] = vidAspectRatio
	}
	if vidResolution != "" {
		videoOptions["resolution"] = vidResolution
	}
	if vidAudio != nil {
		videoOptions["audio"] = *vidAudio
	}
	if imgWM != nil {
		imageOptions["watermark"] = *imgWM
	}
	if vidWM != nil {
		videoOptions["watermark"] = *vidWM
	}

	if len(imageOptions) > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "ImageOptions",
			Inputs:    imageOptions,
		}
		nextID++
	}

	if len(videoOptions) > 0 {
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "VideoOptions",
			Inputs:    videoOptions,
		}
		nextID++
	}

	for _, ref := range task.References {
		if ref.URL == "" {
			continue
		}

		classType := "LoadImage"
		if ref.Type == ReferenceTypeVideo {
			classType = "LoadVideo"
		}

		graph[nodeID(nextID)] = workflow.Node{
			ClassType: classType,
			Inputs: map[string]any{
				"url": ref.URL,
			},
		}
		nextID++
	}

	if task.TTS != nil {
		audioOpts := map[string]any{}
		if task.TTS.Voice != "" {
			audioOpts["voice"] = task.TTS.Voice
		}
		if task.TTS.LanguageType != "" {
			audioOpts["language_type"] = task.TTS.LanguageType
		}
		if task.TTS.Instructions != "" {
			audioOpts["instructions"] = task.TTS.Instructions
		}
		if task.TTS.OptimizeInstructions != nil {
			audioOpts["optimize_instructions"] = *task.TTS.OptimizeInstructions
		}
		if len(audioOpts) > 0 {
			graph[nodeID(nextID)] = workflow.Node{
				ClassType: "AudioOptions",
				Inputs:    audioOpts,
			}
			nextID++
		}
	}

	if task.Music != nil {
		musicOpts := map[string]any{}
		if task.Music.Lyrics != "" {
			musicOpts["lyrics"] = task.Music.Lyrics
		}
		if task.Music.IsInstrumental != nil {
			musicOpts["is_instrumental"] = *task.Music.IsInstrumental
		}
		if task.Music.LyricsOptimizer != nil {
			musicOpts["lyrics_optimizer"] = *task.Music.LyricsOptimizer
		}
		if task.Music.OutputFormat != "" {
			musicOpts["output_format"] = task.Music.OutputFormat
		}
		if task.Music.SampleRate > 0 {
			musicOpts["sample_rate"] = task.Music.SampleRate
		}
		if task.Music.Bitrate > 0 {
			musicOpts["bitrate"] = task.Music.Bitrate
		}
		if task.Music.Format != "" {
			musicOpts["format"] = task.Music.Format
		}
		if len(musicOpts) > 0 {
			graph[nodeID(nextID)] = workflow.Node{
				ClassType: "MusicOptions",
				Inputs:    musicOpts,
			}
			nextID++
		}
	}

	if task.VoiceDesign != nil &&
		task.VoiceDesign.VoicePrompt != "" &&
		task.VoiceDesign.PreviewText != "" &&
		task.VoiceDesign.TargetModel != "" {
		vd := map[string]any{
			"voice_prompt": task.VoiceDesign.VoicePrompt,
			"preview_text": task.VoiceDesign.PreviewText,
			"target_model": task.VoiceDesign.TargetModel,
		}
		if task.VoiceDesign.PreferredName != "" {
			vd["preferred_name"] = task.VoiceDesign.PreferredName
		}
		if task.VoiceDesign.Language != "" {
			vd["language"] = task.VoiceDesign.Language
		}
		if task.VoiceDesign.SampleRate > 0 {
			vd["sample_rate"] = task.VoiceDesign.SampleRate
		}
		if task.VoiceDesign.ResponseFormat != "" {
			vd["response_format"] = task.VoiceDesign.ResponseFormat
		}
		if task.VoiceDesign.OmitPreview {
			vd["omit_preview"] = true
		}
		graph[nodeID(nextID)] = workflow.Node{
			ClassType: "VoiceDesignInput",
			Inputs:    vd,
		}
		nextID++
	}

	return graph
}

// FallbackError records a single engine failure during fallback execution.
type FallbackError struct {
	Engine string
	Err    error
}

func (e FallbackError) Error() string {
	return fmt.Sprintf("engine %q: %v", e.Engine, e.Err)
}

func (e FallbackError) Unwrap() error { return e.Err }

// FallbackResult is the outcome of a fallback-enabled execution.
type FallbackResult struct {
	Engine  string
	Output  Result
	Skipped []FallbackError
}

// ExecuteWithFallback tries each engine in order; the first success wins.
// All engines that fail are recorded in FallbackResult.Skipped.
func (c *Client) ExecuteWithFallback(ctx context.Context, engines []string, graph workflow.Graph, opts ...ExecuteOption) (FallbackResult, error) {
	if len(engines) == 0 {
		return FallbackResult{}, errors.New("aigo: empty engine list")
	}

	var skipped []FallbackError
	for _, name := range engines {
		result, err := c.Execute(ctx, name, graph, opts...)
		if err == nil {
			return FallbackResult{Engine: name, Output: result, Skipped: skipped}, nil
		}
		skipped = append(skipped, FallbackError{Engine: name, Err: err})
	}

	return FallbackResult{Skipped: skipped}, fmt.Errorf("aigo: all %d engines failed", len(engines))
}

// ExecuteTaskWithFallback is the AgentTask variant of ExecuteWithFallback.
func (c *Client) ExecuteTaskWithFallback(ctx context.Context, engines []string, task AgentTask, opts ...ExecuteOption) (FallbackResult, error) {
	return c.ExecuteWithFallback(ctx, engines, BuildGraph(task), opts...)
}

// AsyncResult delivers an asynchronous execution outcome.
type AsyncResult struct {
	Result Result
	Err    error
}

// ExecuteAsync runs Execute in a goroutine and delivers the result on the returned channel.
// The channel is closed after sending exactly one value. Cancelling ctx stops the work.
func (c *Client) ExecuteAsync(ctx context.Context, engineName string, graph workflow.Graph) <-chan AsyncResult {
	ch := make(chan AsyncResult, 1)
	go func() {
		defer close(ch)
		r, err := c.Execute(ctx, engineName, graph)
		ch <- AsyncResult{Result: r, Err: err}
	}()
	return ch
}

// EngineCapabilities returns the capabilities of a registered engine.
// If the engine does not implement Describer, an empty Capability is returned.
func (c *Client) EngineCapabilities(name string) (engine.Capability, error) {
	c.mu.RLock()
	e, ok := c.engines[name]
	c.mu.RUnlock()
	if !ok {
		return engine.Capability{}, fmt.Errorf("%w: %s", ErrEngineNotFound, name)
	}

	if d, ok := e.(engine.Describer); ok {
		return d.Capabilities(), nil
	}
	return engine.Capability{}, nil
}

// AvailableFor returns enabled engine names whose capabilities include the given media type.
// Disabled engines are excluded. Engines that do not implement Describer are included (assumed capable).
func (c *Client) AvailableFor(mediaType string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []string
	for name, e := range c.engines {
		if c.disabled[name] {
			continue
		}
		d, ok := e.(engine.Describer)
		if !ok {
			result = append(result, name)
			continue
		}
		for _, mt := range d.Capabilities().MediaTypes {
			if mt == mediaType {
				result = append(result, name)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

// DryRun checks what would happen without actually executing the task.
// Returns an estimation if the engine implements DryRunner; otherwise returns a basic result
// based on Describer capabilities.
func (c *Client) DryRun(engineName string, task AgentTask) (engine.DryRunResult, error) {
	c.mu.RLock()
	e, ok := c.engines[engineName]
	c.mu.RUnlock()
	if !ok {
		return engine.DryRunResult{}, fmt.Errorf("%w: %s", ErrEngineNotFound, engineName)
	}

	graph := BuildGraph(task)
	if dr, ok := e.(engine.DryRunner); ok {
		return dr.DryRun(graph)
	}

	// Fallback: infer from Describer if available.
	result := engine.DryRunResult{}
	if d, ok := e.(engine.Describer); ok {
		cap := d.Capabilities()
		result.WillPoll = cap.SupportsPoll
	}
	return result, nil
}

func nodeID(v int) string {
	return strconv.Itoa(v)
}

// Submit executes the graph and persists the task for crash recovery.
// If the engine completes synchronously, the result is stored as completed
// and an empty task ID is returned. If the engine returns a remote task ID
// (WaitForCompletion=false), the record is stored as pending and the
// aigo-side task ID is returned for later Resume.
func (c *Client) Submit(ctx context.Context, engineName string, graph workflow.Graph, opts ...ExecuteOption) (string, error) {
	if c.store == nil {
		return "", ErrStoreNotConfigured
	}

	result, err := c.Execute(ctx, engineName, graph, opts...)
	if err != nil {
		return "", err
	}

	now := time.Now()
	rec := TaskRecord{
		ID:         newTaskID(),
		EngineName: engineName,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if result.Kind == OutputPlainText {
		// Engine returned a remote task ID (async, not yet completed).
		rec.RemoteID = result.Value
		rec.Status = TaskStatusPending
	} else {
		// Engine completed synchronously.
		rec.Status = TaskStatusCompleted
		rec.ResultVal = result.Value
		rec.ResultKind = result.Kind
	}

	if err := c.store.Save(rec); err != nil {
		return "", fmt.Errorf("aigo: persist task: %w", err)
	}

	if rec.Status == TaskStatusPending {
		return rec.ID, nil
	}
	return "", nil
}

// Resume polls a previously submitted async task to completion.
// The taskID is the aigo-side ID returned by Submit.
func (c *Client) Resume(ctx context.Context, taskID string) (Result, error) {
	if c.store == nil {
		return Result{}, ErrStoreNotConfigured
	}

	rec, err := c.store.Load(taskID)
	if err != nil {
		return Result{}, err
	}

	if rec.Status == TaskStatusCompleted {
		return Result{
			Value:  rec.ResultVal,
			Kind:   rec.ResultKind,
			Engine: rec.EngineName,
		}, nil
	}
	if rec.Status == TaskStatusFailed {
		return Result{}, fmt.Errorf("aigo: task previously failed: %s", rec.ErrMsg)
	}

	c.mu.RLock()
	e, ok := c.engines[rec.EngineName]
	c.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrEngineNotFound, rec.EngineName)
	}

	resumer, ok := e.(engine.Resumer)
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrResumeNotSupported, rec.EngineName)
	}

	er, resumeErr := resumer.Resume(ctx, rec.RemoteID)

	rec.UpdatedAt = time.Now()
	if resumeErr != nil {
		rec.Status = TaskStatusFailed
		rec.ErrMsg = resumeErr.Error()
		_ = c.store.Save(rec)
		return Result{}, fmt.Errorf("aigo: resume engine %q: %w", rec.EngineName, resumeErr)
	}

	rec.Status = TaskStatusCompleted
	rec.ResultVal = er.Value
	rec.ResultKind = er.Kind
	_ = c.store.Save(rec)

	return Result{
		Value:  er.Value,
		Kind:   er.Kind,
		Engine: rec.EngineName,
	}, nil
}

// RecoverPending returns all tasks that are still pending (not completed or failed).
// Callers should iterate and call Resume for each.
func (c *Client) RecoverPending() ([]TaskRecord, error) {
	if c.store == nil {
		return nil, ErrStoreNotConfigured
	}

	all, err := c.store.All()
	if err != nil {
		return nil, err
	}

	var pending []TaskRecord
	for _, r := range all {
		if r.Status == TaskStatusPending {
			pending = append(pending, r)
		}
	}
	return pending, nil
}
