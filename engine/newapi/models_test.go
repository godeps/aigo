package newapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLookupRoute_KnownModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model    string
		wantR    Route
		wantK    MediaKind
		wantCap  string
	}{
		{"gpt-image-2", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"kling-v2-master", RouteKlingText2Video, KindVideo, "video"},
		{"tts-1", RouteOpenAISpeech, KindSpeech, "tts"},
		{"whisper-1", RouteOpenAITranscriptions, KindSpeech, "asr"},
		{"sora", RouteSoraVideos, KindVideo, "video"},
	}

	for _, tc := range tests {
		route, kind, cap := LookupRoute(tc.model, "")
		if route != tc.wantR {
			t.Errorf("LookupRoute(%q): route = %q, want %q", tc.model, route, tc.wantR)
		}
		if kind != tc.wantK {
			t.Errorf("LookupRoute(%q): kind = %q, want %q", tc.model, kind, tc.wantK)
		}
		if cap != tc.wantCap {
			t.Errorf("LookupRoute(%q): cap = %q, want %q", tc.model, cap, tc.wantCap)
		}
	}
}

func TestLookupRoute_InferFromName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model   string
		wantR   Route
		wantK   MediaKind
		wantCap string
	}{
		{"my-custom-t2v-model", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"acme-text2video-v3", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"acme-i2v-pro", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"acme-image2video", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"custom-video-gen", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"my-tts-model", RouteOpenAISpeech, KindSpeech, "tts"},
		{"speech-synth-v2", RouteOpenAISpeech, KindSpeech, "tts"},
		{"whisper-custom", RouteOpenAITranscriptions, KindSpeech, "asr"},
		{"my-asr-engine", RouteOpenAITranscriptions, KindSpeech, "asr"},
		{"transcription-model", RouteOpenAITranscriptions, KindSpeech, "asr"},
		// new-api aligned image patterns
		{"flux-pro-v2", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"flux.1-schnell", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"gpt-image-1", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"imagen-3.0-generate", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"sdxl-turbo", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"dall-e-custom", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"stable-diffusion-xl", RouteOpenAIImagesGenerations, KindImage, "image"},
	}

	for _, tc := range tests {
		route, kind, cap := LookupRoute(tc.model, "")
		if route != tc.wantR {
			t.Errorf("LookupRoute(%q): route = %q, want %q", tc.model, route, tc.wantR)
		}
		if kind != tc.wantK {
			t.Errorf("LookupRoute(%q): kind = %q, want %q", tc.model, kind, tc.wantK)
		}
		if cap != tc.wantCap {
			t.Errorf("LookupRoute(%q): cap = %q, want %q", tc.model, cap, tc.wantCap)
		}
	}
}

func TestLookupRoute_CapabilityFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model      string
		capability string
		wantR      Route
		wantK      MediaKind
		wantCap    string
	}{
		{"unknown-model-xyz", "video", RouteOpenAIVideoGenerations, KindVideo, "video"},
		{"unknown-model-xyz", "image", RouteOpenAIImagesGenerations, KindImage, "image"},
		{"unknown-model-xyz", "tts", RouteOpenAISpeech, KindSpeech, "tts"},
		{"unknown-model-xyz", "asr", RouteOpenAITranscriptions, KindSpeech, "asr"},
		{"unknown-model-xyz", "image_edit", RouteOpenAIImagesGenerations, KindImage, "image_edit"},
		{"unknown-model-xyz", "video_edit", RouteOpenAIVideoGenerations, KindVideo, "video_edit"},
	}

	for _, tc := range tests {
		route, kind, cap := LookupRoute(tc.model, tc.capability)
		if route != tc.wantR {
			t.Errorf("LookupRoute(%q, %q): route = %q, want %q", tc.model, tc.capability, route, tc.wantR)
		}
		if kind != tc.wantK {
			t.Errorf("LookupRoute(%q, %q): kind = %q, want %q", tc.model, tc.capability, kind, tc.wantK)
		}
		if cap != tc.wantCap {
			t.Errorf("LookupRoute(%q, %q): cap = %q, want %q", tc.model, tc.capability, cap, tc.wantCap)
		}
	}
}

func TestLookupRoute_NoMatch(t *testing.T) {
	t.Parallel()

	route, kind, cap := LookupRoute("completely-unknown-model", "")
	if route != RouteAuto {
		t.Errorf("route = %q, want RouteAuto", route)
	}
	if kind != "" {
		t.Errorf("kind = %q, want empty", kind)
	}
	if cap != "" {
		t.Errorf("cap = %q, want empty", cap)
	}
}

func TestInferFromModelName_Priority(t *testing.T) {
	t.Parallel()

	// "video" substring should match video even if "image" is also in rules
	route, kind, cap := InferFromModelName("my-video-generator")
	if cap != "video" {
		t.Errorf("cap = %q, want video", cap)
	}
	if route != RouteOpenAIVideoGenerations {
		t.Errorf("route = %q", route)
	}
	if kind != KindVideo {
		t.Errorf("kind = %q", kind)
	}
}

func TestInferFromModelName_NoMatch(t *testing.T) {
	t.Parallel()

	route, kind, cap := InferFromModelName("gpt-4o-mini")
	if route != RouteAuto {
		t.Errorf("route = %q, want RouteAuto", route)
	}
	if kind != "" {
		t.Errorf("kind = %q, want empty", kind)
	}
	if cap != "" {
		t.Errorf("cap = %q, want empty", cap)
	}
}

func TestModelsByCapability(t *testing.T) {
	t.Parallel()

	result := ModelsByCapability()
	if len(result["image"]) == 0 {
		t.Error("expected image models")
	}
	if len(result["video"]) == 0 {
		t.Error("expected video models")
	}
	if len(result["tts"]) == 0 {
		t.Error("expected tts models")
	}
	if len(result["asr"]) == 0 {
		t.Error("expected asr models")
	}
}

func TestCapToKindAndRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cap   string
		wantK MediaKind
		wantR Route
	}{
		{"image", KindImage, RouteOpenAIImagesGenerations},
		{"image_edit", KindImage, RouteOpenAIImagesGenerations},
		{"video", KindVideo, RouteOpenAIVideoGenerations},
		{"video_edit", KindVideo, RouteOpenAIVideoGenerations},
		{"tts", KindSpeech, RouteOpenAISpeech},
		{"asr", KindSpeech, RouteOpenAITranscriptions},
		{"unknown_cap", KindImage, RouteOpenAIImagesGenerations},
	}

	for _, tc := range tests {
		kind, route := capToKindAndRoute(tc.cap)
		if kind != tc.wantK {
			t.Errorf("capToKindAndRoute(%q): kind = %q, want %q", tc.cap, kind, tc.wantK)
		}
		if route != tc.wantR {
			t.Errorf("capToKindAndRoute(%q): route = %q, want %q", tc.cap, route, tc.wantR)
		}
	}
}

func TestDiscoverModels_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := modelsResponse{
			Object: "list",
			Data: []ModelEntry{
				{ID: "gpt-image-2", Object: "model"},
				{ID: "custom-t2v-model", Object: "model"},
				{ID: "my-tts", Object: "model"},
				{ID: "some-unknown", Object: "model"},
				{ID: "api-with-cap", Object: "model", Capability: "video"},
				// new-api supported_endpoint_types format
				{ID: "flux-pro", Object: "model", SupportedEndpointTypes: []string{"image-generation", "openai"}},
				{ID: "sora-2", Object: "model", SupportedEndpointTypes: []string{"openai-video"}},
				{ID: "gpt-4o", Object: "model", SupportedEndpointTypes: []string{"openai"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	models, err := DiscoverModels(context.Background(), server.URL, "test-key")
	if err != nil {
		t.Fatal(err)
	}

	// gpt-image-2 -> known -> image
	if !contains(models["image"], "gpt-image-2") {
		t.Errorf("image models = %v, want gpt-image-2", models["image"])
	}
	// custom-t2v-model -> inferred -> video
	if !contains(models["video"], "custom-t2v-model") {
		t.Errorf("video models = %v, want custom-t2v-model", models["video"])
	}
	// my-tts -> inferred -> tts
	if !contains(models["tts"], "my-tts") {
		t.Errorf("tts models = %v, want my-tts", models["tts"])
	}
	// some-unknown -> unknown bucket
	if !contains(models["unknown"], "some-unknown") {
		t.Errorf("unknown models = %v, want some-unknown", models["unknown"])
	}
	// api-with-cap -> explicit capability in response
	if !contains(models["video"], "api-with-cap") {
		t.Errorf("video models = %v, want api-with-cap", models["video"])
	}
	// flux-pro -> supported_endpoint_types "image-generation" -> image
	if !contains(models["image"], "flux-pro") {
		t.Errorf("image models = %v, want flux-pro", models["image"])
	}
	// sora-2 -> supported_endpoint_types "openai-video" -> video
	if !contains(models["video"], "sora-2") {
		t.Errorf("video models = %v, want sora-2", models["video"])
	}
	// gpt-4o -> supported_endpoint_types "openai" only -> no media cap -> unknown
	if !contains(models["unknown"], "gpt-4o") {
		t.Errorf("unknown models = %v, want gpt-4o", models["unknown"])
	}
}

func TestCapFromEndpointTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		types []string
		want  string
	}{
		{[]string{"image-generation", "openai"}, "image"},
		{[]string{"openai-video"}, "video"},
		{[]string{"openai", "openai-response"}, ""},
		{[]string{"image-generation", "openai-video"}, "image"}, // first match wins
		{nil, ""},
		{[]string{}, ""},
	}

	for _, tc := range tests {
		got := capFromEndpointTypes(tc.types)
		if got != tc.want {
			t.Errorf("capFromEndpointTypes(%v) = %q, want %q", tc.types, got, tc.want)
		}
	}
}

func TestDiscoverModels_HTTPError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	_, err := DiscoverModels(context.Background(), server.URL, "bad-key")
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestDiscoverModels_EmptyBaseURL(t *testing.T) {
	t.Parallel()

	_, err := DiscoverModels(context.Background(), "", "key")
	if err == nil {
		t.Fatal("expected error for empty baseURL")
	}
}

func TestDiscoverModels_InvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	_, err := DiscoverModels(context.Background(), server.URL, "key")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDiscoverModels_Timeout(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until context cancelled
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := DiscoverModels(ctx, server.URL, "key")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
