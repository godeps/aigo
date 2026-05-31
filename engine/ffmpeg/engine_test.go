package ffmpeg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not in PATH, skipping")
	}
}

func TestExecuteSFX(t *testing.T) {
	requireFFmpeg(t)
	t.Parallel()

	e := New(Config{Mode: ModeSFX, OutputFormat: "wav"})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "rain on a window"}},
		"2": {ClassType: "SFXOptions", Inputs: map[string]any{"duration": 2}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(result.Value, "data:audio/wav;base64,") {
		t.Fatalf("expected wav data URI, got prefix: %s", result.Value[:40])
	}
}

func TestExecuteSFX_Beep(t *testing.T) {
	requireFFmpeg(t)
	t.Parallel()

	e := New(Config{Mode: ModeSFX})
	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "notification beep"}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(result.Value, "data:audio/mpeg;base64,") {
		t.Fatalf("expected mp3 data URI, got prefix: %s", result.Value[:40])
	}
}

func TestExecuteSFX_MissingPrompt(t *testing.T) {
	t.Parallel()
	e := New(Config{Mode: ModeSFX})
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"duration": 2}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
}

func TestExecuteMix(t *testing.T) {
	requireFFmpeg(t)
	t.Parallel()

	// Serve tiny WAV files via httptest.
	wavHeader := generateSilentWAV()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(wavHeader)
	}))
	defer server.Close()

	e := New(Config{Mode: ModeMix, OutputFormat: "wav"})
	graph := workflow.Graph{
		"1": {ClassType: "AudioMixOptions", Inputs: map[string]any{
			"audio_urls": []any{server.URL + "/a.wav", server.URL + "/b.wav"},
			"volumes":    []any{0.8, 0.3},
			"delays":     []any{0.0, 500.0},
		}},
	}

	result, err := e.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.HasPrefix(result.Value, "data:audio/wav;base64,") {
		t.Fatalf("expected wav data URI, got prefix: %s", result.Value[:40])
	}
}

func TestExecuteMix_MissingURLs(t *testing.T) {
	t.Parallel()
	e := New(Config{Mode: ModeMix})
	graph := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_urls": []any{"http://example.com/a.mp3"},
		}},
	}

	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for < 2 audio URLs")
	}
}

func TestCapabilities_SFX(t *testing.T) {
	t.Parallel()
	e := New(Config{Mode: ModeSFX})
	cap := e.Capabilities()
	found := false
	for _, mt := range cap.MediaTypes {
		if mt == "sfx" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sfx in MediaTypes, got %v", cap.MediaTypes)
	}
}

func TestCapabilities_Mix(t *testing.T) {
	t.Parallel()
	e := New(Config{Mode: ModeMix})
	cap := e.Capabilities()
	found := false
	for _, mt := range cap.MediaTypes {
		if mt == "audio_mix" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected audio_mix in MediaTypes, got %v", cap.MediaTypes)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()
	m := ModelsByCapability()
	if len(m["sfx"]) == 0 {
		t.Fatal("expected sfx models")
	}
	if len(m["audio_mix"]) == 0 {
		t.Fatal("expected audio_mix models")
	}
}

func TestPromptToFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		prompt   string
		contains string
	}{
		{"heavy rain", "brown"},
		{"wind blowing", "pink"},
		{"beep sound", "sine=frequency=880"},
		{"explosion boom", "sine=frequency=60"},
		{"white noise static", "white"},
		{"bell chime", "sine=frequency=1046"},
		{"bird chirp", "sine=frequency=3200"},
		{"laser zap", "sine=frequency=1500"},
		{"fire crackle", "brown"},
		{"glitch error", "violet"},
		{"filter:sine=frequency=999", "sine=frequency=999"},
		{"unknown thing", "pink"},
	}
	for _, c := range cases {
		t.Run(c.prompt, func(t *testing.T) {
			f := promptToFilter(c.prompt)
			if !strings.Contains(f, c.contains) {
				t.Errorf("promptToFilter(%q) = %q, want contains %q", c.prompt, f, c.contains)
			}
		})
	}
}

func TestDownloadToTemp_UnsafeURL(t *testing.T) {
	t.Parallel()
	e := New(Config{Mode: ModeMix})
	graph := workflow.Graph{
		"1": {ClassType: "AudioMixOptions", Inputs: map[string]any{
			"audio_urls": []any{"file:///etc/passwd", "file:///etc/shadow"},
		}},
	}
	_, err := e.Execute(context.Background(), graph)
	if err == nil {
		t.Fatal("expected error for file:// URL")
	}
	if !strings.Contains(err.Error(), "only http/https") {
		t.Fatalf("expected URL scheme error, got: %v", err)
	}
}

// generateSilentWAV creates a minimal valid WAV file (44-byte header + 8820 bytes of silence = 0.1s at 44100Hz mono 16-bit).
func generateSilentWAV() []byte {
	dataSize := 8820
	fileSize := 36 + dataSize
	buf := make([]byte, 44+dataSize)

	copy(buf[0:4], "RIFF")
	putLE32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	putLE32(buf[16:20], 16)
	putLE16(buf[20:22], 1)      // PCM
	putLE16(buf[22:24], 1)      // mono
	putLE32(buf[24:28], 44100)  // sample rate
	putLE32(buf[28:32], 88200)  // byte rate
	putLE16(buf[32:34], 2)      // block align
	putLE16(buf[34:36], 16)     // bits per sample
	copy(buf[36:40], "data")
	putLE32(buf[40:44], uint32(dataSize))
	// rest is zeros (silence)
	return buf
}

func putLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
