package audiogen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

func voiceDesignGraph(voicePrompt, previewText, targetModel string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
			"voice_prompt": voicePrompt,
			"preview_text": previewText,
			"target_model": targetModel,
		}},
	}
}

func voiceDesignGraphWithOptions(voicePrompt, previewText, targetModel string, extra map[string]any) workflow.Graph {
	inputs := map[string]any{
		"voice_prompt": voicePrompt,
		"preview_text": previewText,
		"target_model": targetModel,
	}
	for k, v := range extra {
		inputs[k] = v
	}
	return workflow.Graph{
		"1": {ClassType: "VoiceDesignInput", Inputs: inputs},
	}
}

func TestRunVoiceDesign_Success(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.URL.Path != "/services/audio/tts/customization" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth header")
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"voice":"custom-voice-123","target_model":"qwen3-tts","preview_audio":{"data":"AQID","response_format":"mp3"}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunVoiceDesign(context.Background(), rt, "test-key", "voice-design-v1",
		voiceDesignGraph("a warm female voice", "Hello, how are you?", "qwen3-tts"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out voiceDesignResult
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if out.Type != "qwen-voice-design" {
		t.Fatalf("type = %q, want qwen-voice-design", out.Type)
	}
	if out.Voice != "custom-voice-123" {
		t.Fatalf("voice = %q, want custom-voice-123", out.Voice)
	}
	if out.TargetModel != "qwen3-tts" {
		t.Fatalf("target_model = %q, want qwen3-tts", out.TargetModel)
	}
	if !strings.HasPrefix(out.PreviewAudio, "data:audio/mpeg;base64,") {
		t.Fatalf("preview_audio = %q, want data URI with audio/mpeg", out.PreviewAudio)
	}

	// Verify request payload.
	input, _ := gotPayload["input"].(map[string]any)
	if input["action"] != "create" {
		t.Fatalf("input.action = %v, want create", input["action"])
	}
	if input["voice_prompt"] != "a warm female voice" {
		t.Fatalf("input.voice_prompt = %v", input["voice_prompt"])
	}
	if input["preview_text"] != "Hello, how are you?" {
		t.Fatalf("input.preview_text = %v", input["preview_text"])
	}
	if input["target_model"] != "qwen3-tts" {
		t.Fatalf("input.target_model = %v", input["target_model"])
	}
}

func TestRunVoiceDesign_MissingFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		voicePrompt string
		previewText string
		targetModel string
	}{
		{"missing voice_prompt", "", "preview", "qwen3-tts"},
		{"missing preview_text", "warm voice", "", "qwen3-tts"},
		{"missing target_model", "warm voice", "preview", ""},
		{"all missing", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := voiceDesignGraph(tt.voicePrompt, tt.previewText, tt.targetModel)
			_, err := RunVoiceDesign(context.Background(), nil, "", "", g)
			if err != ierr.ErrMissingVoiceDesign {
				t.Fatalf("expected ErrMissingVoiceDesign, got %v", err)
			}
		})
	}
}

func TestRunVoiceDesign_OmitPreview(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"voice":"custom-voice-456","target_model":"qwen3-tts","preview_audio":{"data":"AQID","response_format":"wav"}}}`))
	}))
	defer srv.Close()

	g := voiceDesignGraphWithOptions("warm voice", "hello", "qwen3-tts", map[string]any{
		"omit_preview": true,
	})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out voiceDesignResult
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if out.PreviewAudio != "" {
		t.Fatalf("preview_audio = %q, want empty (omit_preview=true)", out.PreviewAudio)
	}
	if out.Voice != "custom-voice-456" {
		t.Fatalf("voice = %q, want custom-voice-456", out.Voice)
	}
}

func TestRunVoiceDesign_HTTPError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"code":"InternalError","message":"server error"}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1",
		voiceDesignGraph("warm voice", "hello", "qwen3-tts"))
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestRunVoiceDesign_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"code":"InvalidParameter","message":"bad voice prompt"}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1",
		voiceDesignGraph("warm voice", "hello", "qwen3-tts"))
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "InvalidParameter") {
		t.Fatalf("error %q should contain error code", err.Error())
	}
}

func TestRunVoiceDesign_FallbackTargetModel(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		// Response with empty target_model — should fall back to graph value.
		w.Write([]byte(`{"output":{"voice":"custom-voice-789","target_model":"","preview_audio":{"data":"","response_format":""}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1",
		voiceDesignGraph("warm voice", "hello", "qwen3-tts"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out voiceDesignResult
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if out.TargetModel != "qwen3-tts" {
		t.Fatalf("target_model = %q, want fallback to qwen3-tts", out.TargetModel)
	}
}

func TestRunVoiceDesign_WithOptionalFields(t *testing.T) {
	t.Parallel()
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotPayload)
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"voice":"v1","target_model":"qwen3-tts","preview_audio":{"data":"","response_format":""}}}`))
	}))
	defer srv.Close()

	g := voiceDesignGraphWithOptions("warm voice", "hello", "qwen3-tts", map[string]any{
		"preferred_name":  "MyVoice",
		"language":        "en",
		"sample_rate":     float64(22050),
		"response_format": "opus",
	})
	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	_, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input, _ := gotPayload["input"].(map[string]any)
	if input["preferred_name"] != "MyVoice" {
		t.Fatalf("input.preferred_name = %v, want MyVoice", input["preferred_name"])
	}
	if input["language"] != "en" {
		t.Fatalf("input.language = %v, want en", input["language"])
	}

	params, _ := gotPayload["parameters"].(map[string]any)
	if params == nil {
		t.Fatal("expected parameters in request")
	}
	if params["response_format"] != "opus" {
		t.Fatalf("parameters.response_format = %v, want opus", params["response_format"])
	}
}

func TestRunVoiceDesign_PreviewFormatWav(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"output":{"voice":"v1","target_model":"qwen3-tts","preview_audio":{"data":"AQID","response_format":"wav"}}}`))
	}))
	defer srv.Close()

	rt := &runtime.RT{BaseURL: srv.URL, HTTPClient: http.DefaultClient}
	result, err := RunVoiceDesign(context.Background(), rt, "key", "voice-design-v1",
		voiceDesignGraph("warm voice", "hello", "qwen3-tts"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out voiceDesignResult
	json.Unmarshal([]byte(result), &out)
	if !strings.HasPrefix(out.PreviewAudio, "data:audio/wav;base64,") {
		t.Fatalf("preview_audio = %q, want data URI with audio/wav", out.PreviewAudio)
	}
}
