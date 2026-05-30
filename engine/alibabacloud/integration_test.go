//go:build integration

package alibabacloud

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/godeps/aigo/workflow"
)

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}

func asrGraph(audioURL string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{"audio_url": audioURL}},
	}
}

func ttsGraph(text, voice string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": text}},
		"2": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": voice}},
	}
}

func imageGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

const testAudioURL = "https://dashscope.oss-cn-beijing.aliyuncs.com/audios/2channel_16K.wav"

// --- ASR Tests ---

func TestIntegration_QwenASRFlash(t *testing.T) {
	key := requireEnv(t, "DASHSCOPE_API_KEY")

	eng := New(Config{
		APIKey:            key,
		Model:             ModelQwenASRFlash,
		WaitForCompletion: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := eng.Execute(ctx, asrGraph(testAudioURL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value == "" {
		t.Error("expected non-empty transcription text")
	}
	t.Logf("ASR Flash result: %.200s", result.Value)
}

func TestIntegration_QwenASRFlashFiletrans(t *testing.T) {
	key := requireEnv(t, "DASHSCOPE_API_KEY")

	eng := New(Config{
		APIKey:            key,
		Model:             ModelQwenASRFlashFiletrans,
		WaitForCompletion: true,
		PollInterval:      2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := eng.Execute(ctx, asrGraph(testAudioURL))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value == "" {
		t.Error("expected non-empty transcription text")
	}
	t.Logf("ASR Filetrans result: %.200s", result.Value)
}

// --- TTS Test ---

func TestIntegration_QwenTTSFlash(t *testing.T) {
	key := requireEnv(t, "DASHSCOPE_API_KEY")

	eng := New(Config{
		APIKey:            key,
		Model:             ModelQwenTTSFlash,
		WaitForCompletion: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := eng.Execute(ctx, ttsGraph("你好，这是一段测试语音。", "Cherry"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value == "" {
		t.Error("expected non-empty audio URL or base64")
	}
	t.Logf("TTS result: %.200s", result.Value)
}

// --- Image Generation Test ---

func TestIntegration_QwenImage(t *testing.T) {
	key := requireEnv(t, "DASHSCOPE_API_KEY")

	eng := New(Config{
		APIKey:            key,
		Model:             ModelQwenImage,
		WaitForCompletion: true,
		PollInterval:      2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result, err := eng.Execute(ctx, imageGraph("a simple red circle on white background"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value == "" {
		t.Error("expected non-empty image URL")
	}
	t.Logf("Image result: %.200s", result.Value)
}
