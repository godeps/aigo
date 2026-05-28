package alibabacloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func TestExecuteQwenImageAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/text2image/image-synthesis":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("Authorization header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Errorf("X-DashScope-Async header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"img-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/img-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"img-task","task_status":"SUCCEEDED","results":[{"url":"https://img.example.com/qwen.png"}]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelQwenImage,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只在月球上散步的柴犬"}},
		"2": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1664, "height": 928}},
		"3": {ClassType: "NegativePrompt", Inputs: map[string]any{"negative_prompt": "模糊, 低质量"}},
		"4": {ClassType: "ImageOptions", Inputs: map[string]any{"watermark": false, "n": 1}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/qwen.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelQwenImage {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	if input["prompt"] != "一只在月球上散步的柴犬" {
		t.Fatalf("prompt = %#v", input["prompt"])
	}
	if input["negative_prompt"] != "模糊, 低质量" {
		t.Fatalf("negative_prompt = %#v", input["negative_prompt"])
	}
	parameters := createPayload["parameters"].(map[string]any)
	if parameters["size"] != "1664*928" {
		t.Fatalf("size = %#v", parameters["size"])
	}
}

func TestExecuteWanImageSync(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"type":"image","image":"https://img.example.com/wan.png"}]}}],"finished":true}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelWanImage,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一个有木质门与玻璃窗的花店"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "2K", "watermark": false, "thinking_mode": true}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/wan.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if payload["model"] != ModelWanImage {
		t.Fatalf("model = %#v", payload["model"])
	}
	input := payload["input"].(map[string]any)
	messages := input["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["text"] != "一个有木质门与玻璃窗的花店" {
		t.Fatalf("content text = %#v", content[0])
	}
}

func TestExecuteZImageSync(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"image":"https://img.example.com/zimage.png"}]}}]}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelZImageTurbo,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "A cinematic film still of a rain-soaked neon alley"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1120*1440", "prompt_extend": true}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/zimage.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if payload["model"] != ModelZImageTurbo {
		t.Fatalf("model = %#v", payload["model"])
	}
	input := payload["input"].(map[string]any)
	messages := input["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["text"] != "A cinematic film still of a rain-soaked neon alley" {
		t.Fatalf("content text = %#v", content[0])
	}
	parameters := payload["parameters"].(map[string]any)
	if parameters["size"] != "1120*1440" {
		t.Fatalf("size = %#v", parameters["size"])
	}
	if parameters["prompt_extend"] != true {
		t.Fatalf("prompt_extend = %#v", parameters["prompt_extend"])
	}
}

func TestExecuteQwenImage2Sync(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"type":"image","image":"https://img.example.com/qwen2.png"}]}}]}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenImage2,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只在星空下奔跑的白马"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1024*1024", "watermark": false}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/qwen2.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if payload["model"] != ModelQwenImage2 {
		t.Fatalf("model = %#v", payload["model"])
	}
	input := payload["input"].(map[string]any)
	messages := input["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["text"] != "一只在星空下奔跑的白马" {
		t.Fatalf("content text = %#v", content[0])
	}
}

func TestExecuteQwenImageEditPlusSync(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"type":"image","image":"https://img.example.com/edited.png"}]}}]}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenImageEditPlus,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "把背景替换为海边日落"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/photo.png"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/edited.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if payload["model"] != ModelQwenImageEditPlus {
		t.Fatalf("model = %#v", payload["model"])
	}
	input := payload["input"].(map[string]any)
	messages := input["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content items = %d, want 2 (text + image)", len(content))
	}
	if content[0].(map[string]any)["text"] != "把背景替换为海边日落" {
		t.Fatalf("content text = %#v", content[0])
	}
	if content[1].(map[string]any)["image"] != "https://assets.example.com/photo.png" {
		t.Fatalf("content image = %#v", content[1])
	}
}

func TestExecuteWanVideoT2VAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Errorf("X-DashScope-Async header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"video-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/video-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"video-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/t2v.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelWanTextToVideo,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只机械鲸鱼在云层间游动"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 2, "size": "1280*720", "watermark": false, "audio": false}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/t2v.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelWanTextToVideo {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	if input["prompt"] != "一只机械鲸鱼在云层间游动" {
		t.Fatalf("prompt = %#v", input["prompt"])
	}
	parameters := createPayload["parameters"].(map[string]any)
	if parameters["duration"] != float64(2) {
		t.Fatalf("duration = %#v", parameters["duration"])
	}
	if parameters["size"] != "1280*720" {
		t.Fatalf("size = %#v", parameters["size"])
	}
}

func TestExecuteWanImageToVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"i2v-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/i2v-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"i2v-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/i2v.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelWanImageToVideo,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "让图片中的人物动起来"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/photo.png"}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5, "size": "1280*720"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/i2v.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelWanImageToVideo {
		t.Fatalf("model = %v, want %v", createPayload["model"], ModelWanImageToVideo)
	}
	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 1 {
		t.Fatalf("media len = %d", len(media))
	}
	m0 := media[0].(map[string]any)
	if m0["type"] != "first_frame" || m0["url"] != "https://assets.example.com/photo.png" {
		t.Fatalf("media[0] = %#v", m0)
	}
}

func TestExecuteWanReferenceVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"r2v-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/r2v-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"r2v-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/r2v.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelWanReferenceVideo,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "character1 在街头挥手，character2 从镜头外跑入"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://assets.example.com/role1.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/dog.png"}},
		"4": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 2, "size": "1280*720", "shot_type": "multi", "watermark": false}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/r2v.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 2 {
		t.Fatalf("media len = %d", len(media))
	}
	m0 := media[0].(map[string]any)
	m1 := media[1].(map[string]any)
	if m0["type"] != "first_frame" || m0["url"] != "https://assets.example.com/dog.png" {
		t.Fatalf("media[0] = %#v", m0)
	}
	if m1["type"] != "first_clip" || m1["url"] != "https://assets.example.com/role1.mp4" {
		t.Fatalf("media[1] = %#v", m1)
	}
}

func TestExecuteWanVideoEditAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/edit-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/edit.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelWanVideoEdit,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "把视频中人物的衣服替换为参考图中的风格"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://assets.example.com/input.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/style.png"}},
		"4": {ClassType: "VideoOptions", Inputs: map[string]any{"resolution": "720P", "prompt_extend": true, "watermark": true}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/edit.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 2 {
		t.Fatalf("media len = %d", len(media))
	}
	first := media[0].(map[string]any)
	second := media[1].(map[string]any)
	if first["type"] != "video" || second["type"] != "reference_image" {
		t.Fatalf("media = %#v", media)
	}
}

func TestExecuteWanVideoEditAsyncWithMultipleReferenceImages(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/edit-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/edit.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelWanVideoEdit,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "参考两张图的服装与色彩风格，改造原视频"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://assets.example.com/input.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/style-a.png"}},
		"4": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/style-b.png"}},
		"5": {ClassType: "VideoOptions", Inputs: map[string]any{"resolution": "720P"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/edit.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 3 {
		t.Fatalf("media len = %d", len(media))
	}
	first := media[0].(map[string]any)
	second := media[1].(map[string]any)
	third := media[2].(map[string]any)
	if first["type"] != "video" || second["type"] != "reference_image" || third["type"] != "reference_image" {
		t.Fatalf("media = %#v", media)
	}
	if second["url"] != "https://assets.example.com/style-a.png" || third["url"] != "https://assets.example.com/style-b.png" {
		t.Fatalf("media urls = %#v", media)
	}
}

func TestExecuteQwenTTSNonStream(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"audio":{"url":"https://audio.example.com/out.wav","data":""}}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenTTSFlash,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "你好，欢迎使用语音合成。"}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "Cherry", "language_type": "Chinese"}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://audio.example.com/out.wav" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if payload["model"] != ModelQwenTTSFlash {
		t.Fatalf("model = %#v", payload["model"])
	}
	input := payload["input"].(map[string]any)
	if input["text"] != "你好，欢迎使用语音合成。" || input["voice"] != "Cherry" {
		t.Fatalf("input = %#v", input)
	}
	if input["language_type"] != "Chinese" {
		t.Fatalf("language_type = %#v", input["language_type"])
	}
}

func TestExecuteQwenVoiceDesign(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/audio/tts/customization" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"voice":"qwen-tts-vd-test-voice","target_model":"qwen3-tts-vd-2026-01-26","preview_audio":{"data":"ZmFrZQ==","sample_rate":24000,"response_format":"wav"}}}`))
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenVoiceDesign,
	})

	graph := workflow.Graph{
		"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
			"voice_prompt":   "沉稳的中年男性播音员，语速平稳。",
			"preview_text":   "各位听众晚上好。",
			"target_model":   "qwen3-tts-vd-2026-01-26",
			"preferred_name": "news",
			"language":       "zh",
		}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var decoded struct {
		Type         string `json:"type"`
		Voice        string `json:"voice"`
		TargetModel  string `json:"target_model"`
		PreviewAudio string `json:"preview_audio"`
	}
	if err := json.Unmarshal([]byte(got.Value), &decoded); err != nil {
		t.Fatalf("result json: %v", got.Value)
	}
	if decoded.Type != "qwen-voice-design" || decoded.Voice != "qwen-tts-vd-test-voice" {
		t.Fatalf("decoded = %#v", decoded)
	}
	if !strings.HasPrefix(decoded.PreviewAudio, "data:audio/wav;base64,") {
		t.Fatalf("preview = %q", decoded.PreviewAudio)
	}

	in := payload["input"].(map[string]any)
	if in["action"] != "create" || in["target_model"] != "qwen3-tts-vd-2026-01-26" {
		t.Fatalf("input = %#v", in)
	}
}

func TestExecuteKlingV3TextToVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Errorf("X-DashScope-Async header = %q", got)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-t2v-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/kling-t2v-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-t2v-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/kling-t2v.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelKlingV3Video,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只小猫在月光下奔跑"}},
		"2": {ClassType: "VideoOptions", Inputs: map[string]any{"mode": "pro", "aspect_ratio": "16:9", "duration": 5, "watermark": false}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/kling-t2v.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelKlingV3Video {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	if input["prompt"] != "一只小猫在月光下奔跑" {
		t.Fatalf("prompt = %#v", input["prompt"])
	}
	// t2v should have no media
	if _, hasMedia := input["media"]; hasMedia {
		t.Fatalf("t2v should not have media, got %#v", input["media"])
	}
	parameters := createPayload["parameters"].(map[string]any)
	if parameters["mode"] != "pro" {
		t.Fatalf("mode = %#v", parameters["mode"])
	}
	if parameters["aspect_ratio"] != "16:9" {
		t.Fatalf("aspect_ratio = %#v", parameters["aspect_ratio"])
	}
	if parameters["duration"] != float64(5) {
		t.Fatalf("duration = %#v", parameters["duration"])
	}
}

func TestExecuteKlingV3ImageToVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-i2v-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/kling-i2v-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-i2v-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/kling-i2v.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelKlingV3Video,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "花朵绽放的延时摄影"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/flower.jpg"}},
		"3": {ClassType: "VideoOptions", Inputs: map[string]any{"mode": "std", "duration": 5}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/kling-i2v.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 1 {
		t.Fatalf("media len = %d", len(media))
	}
	first := media[0].(map[string]any)
	if first["type"] != "first_frame" {
		t.Fatalf("media type = %#v, want first_frame", first["type"])
	}
	if first["url"] != "https://example.com/flower.jpg" {
		t.Fatalf("media url = %#v", first["url"])
	}
	parameters := createPayload["parameters"].(map[string]any)
	if parameters["mode"] != "std" {
		t.Fatalf("mode = %#v", parameters["mode"])
	}
}

func TestExecuteKlingV3OmniReferenceVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-omni-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/kling-omni-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"kling-omni-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/kling-omni.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
	}))
	defer server.Close()

	engine := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             ModelKlingV3OmniVideo,
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "角色在街头行走"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/char1.jpg"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/char2.jpg"}},
		"4": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://example.com/ref.mp4"}},
		"5": {ClassType: "VideoOptions", Inputs: map[string]any{"mode": "pro", "aspect_ratio": "16:9", "duration": 10}},
	}

	got, err := engine.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/kling-omni.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}

	if createPayload["model"] != ModelKlingV3OmniVideo {
		t.Fatalf("model = %#v", createPayload["model"])
	}
	input := createPayload["input"].(map[string]any)
	media := input["media"].([]any)
	if len(media) != 3 {
		t.Fatalf("media len = %d, want 3 (2 images + 1 video)", len(media))
	}
	// First image → first_frame, second → last_frame, video → feature
	m0 := media[0].(map[string]any)
	m1 := media[1].(map[string]any)
	m2 := media[2].(map[string]any)
	if m0["type"] != "first_frame" || m1["type"] != "last_frame" || m2["type"] != "feature" {
		t.Fatalf("media types = %v, %v, %v", m0["type"], m1["type"], m2["type"])
	}
	if m2["url"] != "https://example.com/ref.mp4" {
		t.Fatalf("video url = %#v", m2["url"])
	}
}

// TestExecuteMultimodalImage_MultiResult verifies that when the upstream
// returns multiple image URLs in a single choice, Execute exposes all of
// them via Result.Results while keeping Result.Value as the first URL
// (backward-compatible).
func TestExecuteMultimodalImage_MultiResult(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[
			{"type":"image","image":"https://img.example.com/multi-1.png"},
			{"type":"image","image":"https://img.example.com/multi-2.png"},
			{"type":"image","image":"https://img.example.com/multi-3.png"}
		]}}]}}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenImage2,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "三只小猫"}},
		"2": {ClassType: "ImageOptions", Inputs: map[string]any{"n": 3}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/multi-1.png" {
		t.Errorf("Value = %q, want first URL for backward compat", got.Value)
	}
	if len(got.Results) != 3 {
		t.Fatalf("Results len = %d, want 3", len(got.Results))
	}
	wantURLs := []string{
		"https://img.example.com/multi-1.png",
		"https://img.example.com/multi-2.png",
		"https://img.example.com/multi-3.png",
	}
	for i, want := range wantURLs {
		if got.Results[i].Value != want {
			t.Errorf("Results[%d].Value = %q, want %q", i, got.Results[i].Value, want)
		}
		if got.Results[i].Metadata["index"] == "" {
			t.Errorf("Results[%d] missing index metadata", i)
		}
	}
}

func TestResolveModelAliases(t *testing.T) {
	t.Parallel()
	cases := []struct{ input, want string }{
		{"qwen-image-2.0-i2i", ModelQwenImage2},
		{"qwen-image-2.0-t2i", ModelQwenImage2},
		{"qwen-image-i2i", ModelQwenImage},
		{"wan2.7-image-i2i", ModelWanImage},
		{"z-image-turbo-t2i", ModelZImageTurbo},
		{"wan2.7-style", ModelWanVideoEdit},
		{"happyhorse-1.0-style", ModelHappyHorseVideoEdit},
		{"Tripo/Tripo-P1.0-i23d", ModelTripoP1},
		{"Tripo/Tripo-P1.0-mv23d", ModelTripoP1},
		{"Tripo/Tripo-H3.1-t23d", ModelTripoH31},
		{"Tripo/Tripo-H3.1-mv23d", ModelTripoH31},
		// Non-aliases pass through unchanged.
		{ModelWanTextToVideo, ModelWanTextToVideo},
		{ModelQwenImage2, ModelQwenImage2},
		{ModelTripoP1, ModelTripoP1},
		{"", ""},
		{"unknown-model", "unknown-model"},
	}
	for _, tc := range cases {
		if got := ResolveModel(tc.input); got != tc.want {
			t.Errorf("ResolveModel(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExecuteWithModelAlias(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/services/aigc/multimodal-generation/generation" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode body: %v", err)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[{"type":"image","image":"https://img.example.com/alias.png"}]}}]}}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   "qwen-image-2.0-i2i",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "alias test"}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/alias.png" {
		t.Fatalf("Execute() = %q", got.Value)
	}
	if payload["model"] != ModelQwenImage2 {
		t.Fatalf("API model = %v, want %v (canonical)", payload["model"], ModelQwenImage2)
	}
}

// TestExecuteMultimodalImage_SingleResultLeavesResultsNil verifies the n=1
// case keeps Result.Results nil (single-result callers should rely on Value).
func TestExecuteMultimodalImage_SingleResultLeavesResultsNil(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"choices":[{"message":{"content":[
			{"type":"image","image":"https://img.example.com/only.png"}
		]}}]}}`))
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/api/v1",
		Model:   ModelQwenImage2,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "一只猫"}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://img.example.com/only.png" {
		t.Errorf("Value = %q", got.Value)
	}
	if got.Results != nil {
		t.Errorf("Results should be nil when only one item; got %d items", len(got.Results))
	}
}

func TestDefaultWaitForModelWithAlias(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  bool
	}{
		{"qwen-image-2.0-i2i", true},
		{"wan2.7-image-t2i", false},
		{"wan2.7-style", true},
		{"happyhorse-1.0-style", true},
		{"Tripo/Tripo-P1.0-i23d", true},
		{"Tripo/Tripo-H3.1-mv23d", true},
		// Non-alias should still work.
		{ModelWanTextToVideo, true},
		{ModelQwenTTSFlash, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := defaultWaitForModel(tc.model); got != tc.want {
			t.Errorf("defaultWaitForModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestExecuteWithVideoEditStyleAlias(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"style-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/style-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"style-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/style.mp4"}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             "wan2.7-style",
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "将风格迁移到视频"}},
		"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "https://assets.example.com/input.mp4"}},
		"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/style.png"}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://video.example.com/style.mp4" {
		t.Fatalf("Execute() = %q", got.Value)
	}
	if payload["model"] != ModelWanVideoEdit {
		t.Fatalf("API model = %v, want %v (canonical)", payload["model"], ModelWanVideoEdit)
	}
}

func TestExecuteWith3DAlias(t *testing.T) {
	t.Parallel()

	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/3d-generation":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode body: %v", err)
				http.Error(w, "test assertion failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"3d-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/3d-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"3d-task","task_status":"SUCCEEDED","results":{"pbr_model_url":"https://3d.example.com/model.glb"}}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "test assertion failed", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	eng := New(Config{
		APIKey:            "test-key",
		BaseURL:           server.URL + "/api/v1",
		Model:             "Tripo/Tripo-P1.0-i23d",
		WaitForCompletion: true,
		PollInterval:      5 * time.Millisecond,
	})

	graph := workflow.Graph{
		"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://assets.example.com/chair.png"}},
	}

	got, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.Value != "https://3d.example.com/model.glb" {
		t.Fatalf("Execute() = %q", got.Value)
	}
	if payload["model"] != ModelTripoP1 {
		t.Fatalf("API model = %v, want %v (canonical)", payload["model"], ModelTripoP1)
	}
}

func TestCapabilitiesWithAlias(t *testing.T) {
	t.Parallel()
	cases := []struct {
		alias     string
		wantMedia string
	}{
		{"qwen-image-2.0-i2i", "image"},
		{"wan2.7-style", "video"},
		{"Tripo/Tripo-P1.0-i23d", "3d"},
		{"happyhorse-1.0-style", "video"},
	}
	for _, tc := range cases {
		eng := New(Config{Model: tc.alias})
		cap := eng.Capabilities()
		if len(cap.MediaTypes) == 0 || cap.MediaTypes[0] != tc.wantMedia {
			t.Errorf("Capabilities(%q).MediaTypes = %v, want [%s]", tc.alias, cap.MediaTypes, tc.wantMedia)
		}
	}
}
