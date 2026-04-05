package aliyun

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/godeps/aigo/pkg/workflow"
)

func TestExecuteQwenImageAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/text2image/image-synthesis":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("Authorization header = %q", got)
			}
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Fatalf("X-DashScope-Async header = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"img-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/img-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"img-task","task_status":"SUCCEEDED","results":[{"url":"https://img.example.com/qwen.png"}]}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
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
	if got != "https://img.example.com/qwen.png" {
		t.Fatalf("Execute() = %q", got)
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
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
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
	if got != "https://img.example.com/wan.png" {
		t.Fatalf("Execute() = %q", got)
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
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
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
	if got != "https://img.example.com/zimage.png" {
		t.Fatalf("Execute() = %q", got)
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

func TestExecuteWanVideoT2VAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if got := r.Header.Get("X-DashScope-Async"); got != "enable" {
				t.Fatalf("X-DashScope-Async header = %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"video-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/video-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"video-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/t2v.mp4"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
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
	if got != "https://video.example.com/t2v.mp4" {
		t.Fatalf("Execute() = %q", got)
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

func TestExecuteWanReferenceVideoAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"r2v-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/r2v-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"r2v-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/r2v.mp4"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
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
	if got != "https://video.example.com/r2v.mp4" {
		t.Fatalf("Execute() = %q", got)
	}

	input := createPayload["input"].(map[string]any)
	referenceURLs := input["reference_urls"].([]any)
	if len(referenceURLs) != 2 {
		t.Fatalf("reference_urls len = %d", len(referenceURLs))
	}
	if referenceURLs[0] != "https://assets.example.com/role1.mp4" || referenceURLs[1] != "https://assets.example.com/dog.png" {
		t.Fatalf("reference_urls = %#v", referenceURLs)
	}
}

func TestExecuteWanVideoEditAsync(t *testing.T) {
	t.Parallel()

	var createPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/services/aigc/video-generation/video-synthesis":
			if err := json.NewDecoder(r.Body).Decode(&createPayload); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"PENDING"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/edit-task":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"output":{"task_id":"edit-task","task_status":"SUCCEEDED","video_url":"https://video.example.com/edit.mp4"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
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
	if got != "https://video.example.com/edit.mp4" {
		t.Fatalf("Execute() = %q", got)
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
