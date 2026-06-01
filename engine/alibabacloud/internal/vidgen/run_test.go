package vidgen

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/engine/alibabacloud/internal/runtime"
	"github.com/godeps/aigo/workflow"
)

func newTestServer(t *testing.T, taskID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"output": map[string]any{"task_id": taskID},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
}

func newRT(serverURL string) *runtime.RT {
	return &runtime.RT{
		BaseURL:           serverURL,
		HTTPClient:        http.DefaultClient,
		WaitForCompletion: false,
	}
}

// --- Text-to-Video ---

func TestRunTextToVideo_NoWait(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-t2v-001")
	defer server.Close()

	rt := newRT(server.URL)
	g := promptGraph("a beautiful sunset over the ocean")
	taskID, err := RunTextToVideo(context.Background(), rt, "test-key", "wan-t2v", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-t2v-001" {
		t.Errorf("taskID = %q, want %q", taskID, "task-t2v-001")
	}
}

func TestRunTextToVideo_MissingPrompt(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{}}}
	_, err := RunTextToVideo(context.Background(), nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunTextToVideo_WithNegativePrompt(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-t2v-neg")
	defer server.Close()

	rt := newRT(server.URL)
	g := mergeGraphs(
		promptGraph("sunset"),
		optionGraph(map[string]any{"negative_prompt": "blurry"}),
	)
	taskID, err := RunTextToVideo(context.Background(), rt, "test-key", "wan-t2v", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-t2v-neg" {
		t.Errorf("taskID = %q, want %q", taskID, "task-t2v-neg")
	}
}

// --- Reference-to-Video ---

func TestRunReferenceToVideo_NoWait(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-r2v-001")
	defer server.Close()

	rt := newRT(server.URL)
	g := mergeGraphs(
		promptGraph("animate this scene"),
		imageGraph("https://example.com/ref.png"),
	)
	taskID, err := RunReferenceToVideo(context.Background(), rt, "test-key", "wan-r2v", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-r2v-001" {
		t.Errorf("taskID = %q, want %q", taskID, "task-r2v-001")
	}
}

func TestRunReferenceToVideo_MissingPrompt(t *testing.T) {
	t.Parallel()
	g := imageGraph("https://example.com/ref.png")
	_, err := RunReferenceToVideo(context.Background(), nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunReferenceToVideo_MissingImages(t *testing.T) {
	t.Parallel()
	g := promptGraph("animate this")
	_, err := RunReferenceToVideo(context.Background(), nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingReference) {
		t.Fatalf("expected ErrMissingReference, got %v", err)
	}
}

func TestRunReferenceToVideo_MultipleImages(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-r2v-multi")
	defer server.Close()

	rt := newRT(server.URL)
	g := mergeGraphs(
		promptGraph("animate"),
		imageGraph("https://example.com/first.png", "https://example.com/last.png"),
	)
	taskID, err := RunReferenceToVideo(context.Background(), rt, "test-key", "wan-r2v", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-r2v-multi" {
		t.Errorf("taskID = %q, want %q", taskID, "task-r2v-multi")
	}
}

// --- Kling Video ---

func TestRunKlingVideo_NoWait(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-kling-001")
	defer server.Close()

	rt := newRT(server.URL)
	g := promptGraph("cinematic shot of a forest")
	taskID, err := RunKlingVideo(context.Background(), rt, "test-key", "kling/kling-v3-video-generation", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-kling-001" {
		t.Errorf("taskID = %q, want %q", taskID, "task-kling-001")
	}
}

func TestRunKlingVideo_MissingPrompt(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{}}}
	_, err := RunKlingVideo(context.Background(), nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunKlingVideo_WithMedia(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-kling-media")
	defer server.Close()

	rt := newRT(server.URL)
	g := mergeGraphs(
		promptGraph("animate"),
		imageGraph("https://example.com/first.png", "https://example.com/last.png", "https://example.com/refer.png"),
	)
	taskID, err := RunKlingVideo(context.Background(), rt, "test-key", "kling/kling-v3-video-generation", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-kling-media" {
		t.Errorf("taskID = %q, want %q", taskID, "task-kling-media")
	}
}

func TestRunKlingVideo_WithOptions(t *testing.T) {
	t.Parallel()
	server := newTestServer(t, "task-kling-opts")
	defer server.Close()

	rt := newRT(server.URL)
	g := mergeGraphs(
		promptGraph("test"),
		optionGraph(map[string]any{
			"mode":         "pro",
			"aspect_ratio": "16:9",
			"duration":     10,
			"audio":        true,
			"watermark":    false,
		}),
	)
	taskID, err := RunKlingVideo(context.Background(), rt, "test-key", "kling/kling-v3-video-generation", g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if taskID != "task-kling-opts" {
		t.Errorf("taskID = %q, want %q", taskID, "task-kling-opts")
	}
}

// --- buildReferenceMedia ---

func TestBuildReferenceMedia(t *testing.T) {
	t.Parallel()

	t.Run("images_and_videos", func(t *testing.T) {
		t.Parallel()
		g := mergeGraphs(
			imageGraph("https://example.com/first.png", "https://example.com/last.png"),
			workflow.Graph{
				"V": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/clip.mp4"}},
			},
		)
		media, err := buildReferenceMedia(context.Background(), &runtime.RT{}, "", "wan2.7-i2v", g)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(media) != 3 {
			t.Fatalf("expected 3 media items, got %d", len(media))
		}

		// Check types
		types := make(map[string]int)
		for _, m := range media {
			types[m["type"].(string)]++
		}
		if types["first_frame"] != 1 {
			t.Errorf("expected 1 first_frame, got %d", types["first_frame"])
		}
		if types["last_frame"] != 1 {
			t.Errorf("expected 1 last_frame, got %d", types["last_frame"])
		}
		if types["first_clip"] != 1 {
			t.Errorf("expected 1 first_clip, got %d", types["first_clip"])
		}
	})

	t.Run("empty_graph", func(t *testing.T) {
		t.Parallel()
		g := promptGraph("test")
		media, err := buildReferenceMedia(context.Background(), &runtime.RT{}, "", "wan2.7-i2v", g)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(media) != 0 {
			t.Errorf("expected empty media, got %d items", len(media))
		}
	})
}

// --- buildKlingMedia ---

func TestBuildKlingMedia(t *testing.T) {
	t.Parallel()

	t.Run("three_images_one_video", func(t *testing.T) {
		t.Parallel()
		g := mergeGraphs(
			imageGraph("https://example.com/a.png", "https://example.com/b.png", "https://example.com/c.png"),
			workflow.Graph{
				"V": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/v.mp4"}},
			},
		)
		media := buildKlingMedia(g)
		if len(media) != 4 {
			t.Fatalf("expected 4 media items, got %d", len(media))
		}

		types := make(map[string]int)
		for _, m := range media {
			types[m["type"].(string)]++
		}
		if types["first_frame"] != 1 {
			t.Errorf("expected 1 first_frame, got %d", types["first_frame"])
		}
		if types["last_frame"] != 1 {
			t.Errorf("expected 1 last_frame, got %d", types["last_frame"])
		}
		if types["refer"] != 1 {
			t.Errorf("expected 1 refer, got %d", types["refer"])
		}
		if types["feature"] != 1 {
			t.Errorf("expected 1 feature, got %d", types["feature"])
		}
	})

	t.Run("empty_graph", func(t *testing.T) {
		t.Parallel()
		g := promptGraph("test")
		media := buildKlingMedia(g)
		if len(media) != 0 {
			t.Errorf("expected empty media, got %d items", len(media))
		}
	})
}

// --- buildKlingParameters ---

func TestBuildKlingParameters(t *testing.T) {
	t.Parallel()

	t.Run("all_options", func(t *testing.T) {
		t.Parallel()
		g := optionGraph(map[string]any{
			"mode":         "pro",
			"aspect_ratio": "16:9",
			"duration":     10,
			"audio":        true,
			"watermark":    false,
		})
		p := buildKlingParameters(g)
		if p["mode"] != "pro" {
			t.Errorf("mode = %v, want pro", p["mode"])
		}
		if p["aspect_ratio"] != "16:9" {
			t.Errorf("aspect_ratio = %v, want 16:9", p["aspect_ratio"])
		}
		if p["duration"] != 10 {
			t.Errorf("duration = %v, want 10", p["duration"])
		}
		if p["audio"] != true {
			t.Errorf("audio = %v, want true", p["audio"])
		}
		if p["watermark"] != false {
			t.Errorf("watermark = %v, want false", p["watermark"])
		}
	})

	t.Run("empty_options", func(t *testing.T) {
		t.Parallel()
		g := promptGraph("test")
		p := buildKlingParameters(g)
		if len(p) != 0 {
			t.Errorf("expected empty params, got %v", p)
		}
	})
}
