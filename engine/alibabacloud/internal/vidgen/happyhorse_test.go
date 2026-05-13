package vidgen

import (
	"errors"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
	"github.com/godeps/aigo/workflow"
)

func promptGraph(prompt string) workflow.Graph {
	return workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": prompt}},
	}
}

func imageGraph(urls ...string) workflow.Graph {
	g := workflow.Graph{}
	for i, u := range urls {
		id := string(rune('A' + i))
		g[id] = workflow.Node{ClassType: "LoadImage", Inputs: map[string]any{"url": u}}
	}
	return g
}

func mergeGraphs(graphs ...workflow.Graph) workflow.Graph {
	merged := workflow.Graph{}
	for _, g := range graphs {
		for k, v := range g {
			merged[k] = v
		}
	}
	return merged
}

func optionGraph(opts map[string]any) workflow.Graph {
	return workflow.Graph{
		"opt": {ClassType: "Options", Inputs: opts},
	}
}

func TestBuildHappyHorseParams_WithRatio(t *testing.T) {
	g := optionGraph(map[string]any{
		"resolution": "1080P",
		"ratio":      "16:9",
		"duration":   10,
		"watermark":  false,
		"seed":       42,
	})
	p := buildHappyHorseParams(g, true, false)

	if p["resolution"] != "1080P" {
		t.Errorf("resolution = %v, want 1080P", p["resolution"])
	}
	if p["ratio"] != "16:9" {
		t.Errorf("ratio = %v, want 16:9", p["ratio"])
	}
	if p["duration"] != 10 {
		t.Errorf("duration = %v, want 10", p["duration"])
	}
	if p["watermark"] != false {
		t.Errorf("watermark = %v, want false", p["watermark"])
	}
	if p["seed"] != 42 {
		t.Errorf("seed = %v, want 42", p["seed"])
	}
}

func TestBuildHappyHorseParams_NoRatio(t *testing.T) {
	g := optionGraph(map[string]any{"resolution": "720P", "ratio": "9:16"})
	p := buildHappyHorseParams(g, false, false)

	if p["resolution"] != "720P" {
		t.Errorf("resolution = %v, want 720P", p["resolution"])
	}
	if _, has := p["ratio"]; has {
		t.Errorf("ratio should not be present when includeRatio=false")
	}
}

func TestBuildHappyHorseParams_AudioSetting(t *testing.T) {
	g := optionGraph(map[string]any{"audio_setting": "origin"})
	p := buildHappyHorseParams(g, false, true)

	if p["audio_setting"] != "origin" {
		t.Errorf("audio_setting = %v, want origin", p["audio_setting"])
	}
}

func TestBuildHappyHorseParams_AudioSettingOmitted(t *testing.T) {
	g := optionGraph(map[string]any{"audio_setting": "auto"})
	p := buildHappyHorseParams(g, false, false)

	if _, has := p["audio_setting"]; has {
		t.Errorf("audio_setting should not be present when includeAudioSetting=false")
	}
}

func TestBuildHappyHorseParams_Empty(t *testing.T) {
	g := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}}}
	p := buildHappyHorseParams(g, true, true)

	if len(p) != 0 {
		t.Errorf("expected empty params for graph with no options, got %v", p)
	}
}

func TestRunHappyHorseTextToVideo_MissingPrompt(t *testing.T) {
	g := workflow.Graph{"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{}}}
	_, err := RunHappyHorseTextToVideo(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunHappyHorseImageToVideo_MissingImage(t *testing.T) {
	g := promptGraph("animate this")
	_, err := RunHappyHorseImageToVideo(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingReference) {
		t.Fatalf("expected ErrMissingReference, got %v", err)
	}
}

func TestRunHappyHorseReferenceToVideo_MissingImage(t *testing.T) {
	g := promptGraph("generate video")
	_, err := RunHappyHorseReferenceToVideo(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingReference) {
		t.Fatalf("expected ErrMissingReference, got %v", err)
	}
}

func TestRunHappyHorseReferenceToVideo_MissingPrompt(t *testing.T) {
	g := imageGraph("https://example.com/a.png")
	_, err := RunHappyHorseReferenceToVideo(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}

func TestRunHappyHorseReferenceToVideo_TooManyImages(t *testing.T) {
	urls := make([]string, 10)
	for i := range urls {
		urls[i] = "https://example.com/img" + string(rune('0'+i)) + ".png"
	}
	g := mergeGraphs(promptGraph("test"), imageGraph(urls...))
	_, err := RunHappyHorseReferenceToVideo(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrTooManyHappyHorseImages) {
		t.Fatalf("expected ErrTooManyHappyHorseImages, got %v", err)
	}
}

func TestRunHappyHorseVideoEdit_MissingMedia(t *testing.T) {
	g := promptGraph("edit the video")
	_, err := RunHappyHorseVideoEdit(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingReference) {
		t.Fatalf("expected ErrMissingReference, got %v", err)
	}
}
