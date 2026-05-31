// Package ffmpeg implements engine.Engine for local audio processing via ffmpeg.
//
// Two modes: "sfx" generates basic sound effects using lavfi sources,
// "mix" combines multiple audio tracks with volume/delay controls.
// Requires ffmpeg binary in PATH.
package ffmpeg

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"net/url"
	"strings"

	ffmpeggo "github.com/u2takey/ffmpeg-go"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	ModeSFX = "sfx"
	ModeMix = "mix"
)

const maxOutputSize = 100 * 1024 * 1024 // 100 MB

var (
	ErrMissingPrompt    = errors.New("ffmpeg: missing prompt for SFX generation")
	ErrMissingAudioURLs = errors.New("ffmpeg: audio_urls is required (minimum 2)")
	ErrUnsafeURL        = errors.New("ffmpeg: only http/https URLs are allowed")
	ErrOutputTooLarge   = errors.New("ffmpeg: output exceeds 100 MB size limit")
)

// Config configures the FFmpeg engine.
type Config struct {
	Mode         string // "sfx" or "mix"
	OutputFormat string // "mp3" (default), "wav", "flac"
	HTTPClient   *http.Client
}

// Engine implements engine.Engine for local ffmpeg operations.
type Engine struct {
	mode         string
	outputFormat string
	httpClient   *http.Client
}

// New creates an FFmpeg engine instance.
func New(cfg Config) *Engine {
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		mode = ModeMix
	}
	format := strings.TrimSpace(cfg.OutputFormat)
	if format == "" {
		format = "mp3"
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	return &Engine{mode: mode, outputFormat: format, httpClient: hc}
}

// Execute dispatches to SFX generation or audio mixing based on mode.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: validate graph: %w", err)
	}

	format := e.outputFormat
	if f, ok := resolve.StringOption(g, "format"); ok && f != "" {
		format = f
	}

	switch e.mode {
	case ModeSFX:
		return e.executeSFX(ctx, g, format)
	case ModeMix:
		return e.executeMix(ctx, g, format)
	default:
		return engine.Result{}, fmt.Errorf("ffmpeg: unknown mode %q", e.mode)
	}
}

func (e *Engine) executeSFX(ctx context.Context, g workflow.Graph, format string) (engine.Result, error) {
	prompt, _ := resolve.ExtractPrompt(g)
	if strings.TrimSpace(prompt) == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	duration := 3.0
	if d, ok := resolve.Float64Option(g, "duration"); ok && d > 0 {
		duration = d
	}

	filter := promptToFilter(prompt)

	outFile, err := os.CreateTemp("", "ffmpeg-sfx-*."+format)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: create temp file: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	stream := ffmpeggo.Input(filter, ffmpeggo.KwArgs{"f": "lavfi"}).
		Output(outPath, ffmpeggo.KwArgs{"t": fmt.Sprintf("%.1f", duration)}).
		OverWriteOutput().
		Silent(true)
	stream.Context = ctx

	if err := stream.Run(); err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: generate sfx: %w", err)
	}

	return fileToDataURI(outPath, format)
}

func (e *Engine) executeMix(ctx context.Context, g workflow.Graph, format string) (engine.Result, error) {
	urls, ok := resolve.StringSliceOption(g, "audio_urls")
	if !ok || len(urls) < 2 {
		return engine.Result{}, ErrMissingAudioURLs
	}

	volumes, _ := resolve.Float64SliceOption(g, "volumes")
	delays, _ := resolve.Float64SliceOption(g, "delays")

	maxDuration := 0.0
	if d, ok := resolve.Float64Option(g, "duration"); ok && d > 0 {
		maxDuration = d
	}

	tmpFiles := make([]string, 0, len(urls))
	defer func() {
		for _, f := range tmpFiles {
			os.Remove(f)
		}
	}()

	for _, u := range urls {
		path, err := downloadToTemp(ctx, e.httpClient, u)
		if err != nil {
			return engine.Result{}, fmt.Errorf("ffmpeg: download %q: %w", u, err)
		}
		tmpFiles = append(tmpFiles, path)
	}

	outFile, err := os.CreateTemp("", "ffmpeg-mix-*."+format)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: create temp file: %w", err)
	}
	outPath := outFile.Name()
	outFile.Close()
	defer os.Remove(outPath)

	// Build per-track streams with volume/delay filters via ffmpeg-go API.
	tracks := make([]*ffmpeggo.Stream, len(tmpFiles))
	for i, f := range tmpFiles {
		s := ffmpeggo.Input(f)

		if i < len(delays) && delays[i] > 0 {
			ms := int(delays[i])
			s = s.Filter("adelay", ffmpeggo.Args{fmt.Sprintf("%d|%d", ms, ms)})
		}
		if i < len(volumes) && volumes[i] >= 0 && volumes[i] != 1.0 {
			s = s.Filter("volume", ffmpeggo.Args{fmt.Sprintf("%.2f", volumes[i])})
		}

		tracks[i] = s
	}

	// Combine all tracks with amix filter.
	mixed := ffmpeggo.Filter(tracks, "amix", nil, ffmpeggo.KwArgs{
		"inputs":   len(tracks),
		"duration": "longest",
	})

	outKwargs := ffmpeggo.KwArgs{}
	if maxDuration > 0 {
		outKwargs["t"] = fmt.Sprintf("%.1f", maxDuration)
	}

	stream := ffmpeggo.OutputContext(ctx, []*ffmpeggo.Stream{mixed}, outPath, outKwargs).
		OverWriteOutput().
		Silent(true)

	if err := stream.Run(); err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: mix audio: %w", err)
	}

	return fileToDataURI(outPath, format)
}

func promptToFilter(prompt string) string {
	p := strings.ToLower(prompt)

	if strings.HasPrefix(p, "filter:") {
		return strings.TrimSpace(strings.TrimPrefix(p, "filter:"))
	}

	switch {
	// Nature / ambient — layered noise with EQ and modulation
	case containsAny(p, "rain", "water", "ocean", "wave", "stream", "river"):
		return "anoisesrc=color=brown:amplitude=0.4,lowpass=f=800,tremolo=f=0.3:d=0.4"
	case containsAny(p, "wind", "breeze", "air", "gust"):
		return "anoisesrc=color=pink:amplitude=0.25,bandpass=f=400:width_type=o:w=2,tremolo=f=0.15:d=0.6"
	case containsAny(p, "fire", "crackle", "flame", "campfire"):
		return "anoisesrc=color=brown:amplitude=0.3,bandpass=f=1200:width_type=o:w=3,tremolo=f=8:d=0.7"
	case containsAny(p, "bird", "chirp", "tweet", "cricket", "insect"):
		return "sine=frequency=3200:sample_rate=44100,tremolo=f=12:d=0.9,afade=t=in:d=0.02,afade=t=out:st=0.15:d=0.1"

	// Mechanical / impact — envelope-shaped tones
	case containsAny(p, "bass", "boom", "rumble", "thunder", "explosion", "cannon"):
		return "sine=frequency=60:sample_rate=44100,afade=t=in:d=0.01,afade=t=out:st=0.1:d=0.8,aecho=0.6:0.3:40:0.4"
	case containsAny(p, "click", "tap", "knock", "tick"):
		return "sine=frequency=1200:sample_rate=44100,afade=t=in:d=0.001,afade=t=out:st=0.01:d=0.05"
	case containsAny(p, "metal", "clang", "anvil", "hammer"):
		return "sine=frequency=2400:sample_rate=44100,afade=t=in:d=0.001,afade=t=out:st=0.05:d=0.4,aecho=0.8:0.5:20:0.3"
	case containsAny(p, "engine", "motor", "machine", "vibrat"):
		return "sine=frequency=120:sample_rate=44100,tremolo=f=6:d=0.5,lowpass=f=500"

	// Tonal / musical — shaped attacks and decays
	case containsAny(p, "bell", "chime", "ring", "ding"):
		return "sine=frequency=1046:sample_rate=44100,afade=t=in:d=0.005,afade=t=out:st=0.1:d=1.5,aecho=0.6:0.3:60:0.25"
	case containsAny(p, "beep", "alert", "notification", "ping"):
		return "sine=frequency=880:sample_rate=44100,afade=t=in:d=0.005,afade=t=out:st=0.15:d=0.1"
	case containsAny(p, "siren", "alarm", "emergency"):
		return "sine=frequency=660:sample_rate=44100,tremolo=f=3:d=0.8"
	case containsAny(p, "whistle", "flute", "pipe"):
		return "sine=frequency=1568:sample_rate=44100,tremolo=f=5:d=0.3,afade=t=in:d=0.05,afade=t=out:st=0.3:d=0.2"
	case containsAny(p, "tone", "hum", "drone", "buzz"):
		return "sine=frequency=440:sample_rate=44100,tremolo=f=0.5:d=0.2"

	// Electronic / noise — filtered and modulated
	case containsAny(p, "static", "noise", "hiss", "tv", "radio"):
		return "anoisesrc=color=white:amplitude=0.15,highpass=f=2000"
	case containsAny(p, "glitch", "digital", "error", "corrupt"):
		return "anoisesrc=color=violet:amplitude=0.25,tremolo=f=20:d=0.9,highpass=f=1000"
	case containsAny(p, "sweep", "whoosh", "swish", "swing"):
		return "sine=frequency=200:sample_rate=44100,afade=t=in:d=0.01,afade=t=out:st=0.1:d=0.5,aecho=0.5:0.3:30:0.2"
	case containsAny(p, "laser", "zap", "beam", "phaser"):
		return "sine=frequency=1500:sample_rate=44100,afade=t=in:d=0.001,afade=t=out:st=0.02:d=0.3,aecho=0.8:0.4:10:0.5"
	case containsAny(p, "bubble", "pop", "drip", "drop"):
		return "sine=frequency=600:sample_rate=44100,afade=t=in:d=0.001,afade=t=out:st=0.02:d=0.15"

	default:
		return "anoisesrc=color=pink:amplitude=0.2,lowpass=f=1000"
	}
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func downloadToTemp(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("ffmpeg: invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrUnsafeURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "ffmpeg-dl-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	tmp.Close()
	return tmp.Name(), nil
}

func fileToDataURI(path, format string) (engine.Result, error) {
	info, err := os.Stat(path)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: stat output: %w", err)
	}
	if info.Size() > maxOutputSize {
		return engine.Result{}, ErrOutputTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: read output: %w", err)
	}
	mime := "audio/mpeg"
	switch format {
	case "wav":
		mime = "audio/wav"
	case "flac":
		mime = "audio/flac"
	}
	uri := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
	return engine.Result{Value: uri, Kind: engine.OutputDataURI}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	switch e.mode {
	case ModeSFX:
		return engine.Capability{
			MediaTypes:   []string{"audio", "sfx"},
			Models:       []string{ModelFFmpegSFX},
			SupportsSync: true,
		}
	default:
		return engine.Capability{
			MediaTypes:   []string{"audio", "audio_mix"},
			Models:       []string{ModelFFmpegMix},
			SupportsSync: true,
		}
	}
}

// ConfigSchema returns configuration fields for the FFmpeg engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "mode", Label: "Mode", Type: "string", Required: true, Description: "Operation mode: sfx or mix", Default: "mix"},
		{Key: "outputFormat", Label: "Output Format", Type: "string", Description: "Output audio format (mp3, wav, flac)", Default: "mp3"},
	}
}

// ModelsByCapability returns models grouped by capability.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"sfx":       {ModelFFmpegSFX},
		"audio_mix": {ModelFFmpegMix},
	}
}
