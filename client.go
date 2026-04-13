package aigo

import (
	"context"
	"errors"
	"fmt"
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

// Middleware wraps an engine to add cross-cutting behavior (logging, timing, retry, etc.).
type Middleware func(name string, next engine.Engine) engine.Engine

// Client routes a workflow graph to a registered execution engine.
type Client struct {
	mu         sync.RWMutex
	engines    map[string]engine.Engine
	middleware []Middleware
}

// NewClient creates a new SDK client.
func NewClient() *Client {
	return &Client{
		engines: make(map[string]engine.Engine),
	}
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
func (c *Client) Execute(ctx context.Context, engineName string, graph workflow.Graph, opts ...ExecuteOption) (Result, error) {
	c.mu.RLock()
	e, ok := c.engines[engineName]
	c.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrEngineNotFound, engineName)
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
func (c *Client) ExecuteTaskAuto(ctx context.Context, selector Selector, task AgentTask) (RoutedResult, error) {
	if selector == nil {
		return RoutedResult{}, errors.New("aigo: selector is nil")
	}

	engines := c.EngineNames()
	selection, err := selector.SelectEngine(ctx, task, engines)
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

// EngineNames returns registered engine names in deterministic order.
func (c *Client) EngineNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0, len(c.engines))
	for name := range c.engines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

// AvailableFor returns registered engine names whose capabilities include the given media type.
// Engines that do not implement Describer are included (assumed capable).
func (c *Client) AvailableFor(mediaType string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []string
	for name, e := range c.engines {
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
