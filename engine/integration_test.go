//go:build integration

package engine_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/gemini"
	"github.com/godeps/aigo/engine/gpt4o"
	"github.com/godeps/aigo/engine/hailuo"
	"github.com/godeps/aigo/engine/kling"
	"github.com/godeps/aigo/engine/recraft"
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

func testGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func assertResult(t *testing.T, result engine.Result, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Value == "" {
		t.Error("expected non-empty result value")
	}
}

func TestHailuo_Integration(t *testing.T) {
	key := requireEnv(t, "HAILUO_API_KEY")
	eng := hailuo.New(hailuo.Config{
		APIKey:            key,
		WaitForCompletion: false,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := eng.Execute(ctx, testGraph("a cat walking in the garden"))
	assertResult(t, result, err)
	t.Logf("task_id: %s", result.Value)
}

func TestKling_Integration(t *testing.T) {
	key := requireEnv(t, "KLING_API_KEY")
	eng := kling.New(kling.Config{
		APIKey:            key,
		WaitForCompletion: false,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := eng.Execute(ctx, testGraph("a sunset over the ocean"))
	assertResult(t, result, err)
	t.Logf("task_id: %s", result.Value)
}

func TestRecraft_Integration(t *testing.T) {
	key := requireEnv(t, "RECRAFT_API_KEY")
	eng := recraft.New(recraft.Config{
		APIKey: key,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := eng.Execute(ctx, testGraph("a minimalist logo"))
	assertResult(t, result, err)
	t.Logf("url: %s", result.Value)
}

func TestGemini_Integration(t *testing.T) {
	key := requireEnv(t, "GEMINI_API_KEY")
	eng := gemini.New(gemini.Config{
		APIKey: key,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := eng.Execute(ctx, testGraph("Explain quantum computing in 2 sentences"))
	assertResult(t, result, err)
	t.Logf("response: %.200s", result.Value)
}

func TestGPT4o_Integration(t *testing.T) {
	key := requireEnv(t, "OPENAI_API_KEY")
	eng := gpt4o.New(gpt4o.Config{
		APIKey: key,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := eng.Execute(ctx, testGraph("What is 2+2? Reply with just the number."))
	assertResult(t, result, err)
	t.Logf("response: %s", result.Value)
}
