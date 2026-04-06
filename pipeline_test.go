package aigo

import (
	"context"
	"testing"
)

func TestPipeline_TwoSteps(t *testing.T) {
	t.Parallel()

	c := NewClient()
	_ = c.RegisterEngine("img", stubEngine{result: "https://img.example.com/1.png"})
	_ = c.RegisterEngine("vid", stubEngine{result: "https://vid.example.com/1.mp4"})

	p := NewPipeline("img", AgentTask{Prompt: "a cat"}).
		Then(func(prev Result) (AgentTask, string) {
			return AgentTask{
				Prompt:     "animate this image",
				References: []ReferenceAsset{{Type: ReferenceTypeImage, URL: prev.Value}},
			}, "vid"
		})

	results, err := c.ExecutePipeline(context.Background(), p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Value != "https://img.example.com/1.png" {
		t.Fatalf("step 0 = %q", results[0].Value)
	}
	if results[1].Value != "https://vid.example.com/1.mp4" {
		t.Fatalf("step 1 = %q", results[1].Value)
	}
}

func TestPipeline_FirstStepFails(t *testing.T) {
	t.Parallel()

	c := NewClient()
	// No engine registered — Execute will fail.
	p := NewPipeline("missing", AgentTask{Prompt: "fail"})
	_, err := c.ExecutePipeline(context.Background(), p)
	if err == nil {
		t.Fatal("expected error")
	}
}
