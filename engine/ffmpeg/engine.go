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
	"os/exec"
	"strings"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	ModeSFX = "sfx"
	ModeMix = "mix"
)

var (
	ErrMissingPrompt    = errors.New("ffmpeg: missing prompt for SFX generation")
	ErrMissingAudioURLs = errors.New("ffmpeg: audio_urls is required (minimum 2)")
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

	err = exec.CommandContext(ctx, "ffmpeg",
		"-f", "lavfi", "-i", filter,
		"-t", fmt.Sprintf("%.1f", duration),
		"-y", outPath,
	).Run()
	if err != nil {
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

	filterParts := make([]string, 0, len(tmpFiles))
	for i := range tmpFiles {
		label := fmt.Sprintf("[%d:a]", i)
		var filters []string

		if i < len(delays) && delays[i] > 0 {
			ms := int(delays[i])
			filters = append(filters, fmt.Sprintf("adelay=%d|%d", ms, ms))
		}
		if i < len(volumes) && volumes[i] >= 0 && volumes[i] != 1.0 {
			filters = append(filters, fmt.Sprintf("volume=%.2f", volumes[i]))
		}

		outLabel := fmt.Sprintf("[a%d]", i)
		if len(filters) > 0 {
			filterParts = append(filterParts, fmt.Sprintf("%s%s%s", label, strings.Join(filters, ","), outLabel))
		} else {
			filterParts = append(filterParts, fmt.Sprintf("%sanull%s", label, outLabel))
		}
	}

	mixLabels := make([]string, len(tmpFiles))
	for i := range tmpFiles {
		mixLabels[i] = fmt.Sprintf("[a%d]", i)
	}
	filterParts = append(filterParts, fmt.Sprintf("%samix=inputs=%d:duration=longest[out]",
		strings.Join(mixLabels, ""), len(tmpFiles)))

	filterComplex := strings.Join(filterParts, ";")

	args := []string{"-y"}
	for _, f := range tmpFiles {
		args = append(args, "-i", f)
	}
	args = append(args, "-filter_complex", filterComplex, "-map", "[out]")
	if maxDuration > 0 {
		args = append(args, "-t", fmt.Sprintf("%.1f", maxDuration))
	}
	args = append(args, outPath)

	err = exec.CommandContext(ctx, "ffmpeg", args...).Run()
	if err != nil {
		return engine.Result{}, fmt.Errorf("ffmpeg: mix audio: %w", err)
	}

	return fileToDataURI(outPath, format)
}

func promptToFilter(prompt string) string {
	p := strings.ToLower(prompt)
	switch {
	case containsAny(p, "rain", "water", "ocean", "wave", "stream"):
		return "anoisesrc=color=brown:amplitude=0.3"
	case containsAny(p, "wind", "breeze", "air"):
		return "anoisesrc=color=pink:amplitude=0.2"
	case containsAny(p, "static", "noise", "hiss", "tv"):
		return "anoisesrc=color=white:amplitude=0.15"
	case containsAny(p, "beep", "alert", "notification", "ping"):
		return "sine=frequency=880:sample_rate=44100"
	case containsAny(p, "bass", "boom", "rumble", "thunder", "explosion"):
		return "sine=frequency=60:sample_rate=44100"
	case containsAny(p, "tone", "hum", "drone"):
		return "sine=frequency=440:sample_rate=44100"
	case containsAny(p, "click", "tap", "knock"):
		return "sine=frequency=1200:sample_rate=44100"
	case containsAny(p, "siren", "alarm"):
		return "sine=frequency=660:sample_rate=44100"
	default:
		return "anoisesrc=color=pink:amplitude=0.2"
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

func downloadToTemp(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
