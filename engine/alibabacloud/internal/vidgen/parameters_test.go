package vidgen

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestBuildParameters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		graph            workflow.Graph
		preferResolution bool
		wantKey          string
		wantValue        any
	}{
		{
			name:             "size_with_preferResolution_false",
			graph:            optionGraph(map[string]any{"size": "1280*720"}),
			preferResolution: false,
			wantKey:          "size",
			wantValue:        "1280*720",
		},
		{
			name:             "size_normalized_x_to_asterisk",
			graph:            optionGraph(map[string]any{"size": "1280x720"}),
			preferResolution: false,
			wantKey:          "size",
			wantValue:        "1280*720",
		},
		{
			name:             "resolution_derived_from_size_with_preferResolution_true",
			graph:            optionGraph(map[string]any{"size": "1280*720"}),
			preferResolution: true,
			wantKey:          "resolution",
			wantValue:        "720P",
		},
		{
			name:             "explicit_resolution_with_preferResolution_true",
			graph:            optionGraph(map[string]any{"resolution": "1080P"}),
			preferResolution: true,
			wantKey:          "resolution",
			wantValue:        "1080P",
		},
		{
			name:             "duration_present",
			graph:            optionGraph(map[string]any{"duration": 5}),
			preferResolution: false,
			wantKey:          "duration",
			wantValue:        5,
		},
		{
			name:             "watermark_present",
			graph:            optionGraph(map[string]any{"watermark": true}),
			preferResolution: false,
			wantKey:          "watermark",
			wantValue:        true,
		},
		{
			name:             "audio_with_preferResolution_false",
			graph:            optionGraph(map[string]any{"audio": false}),
			preferResolution: false,
			wantKey:          "audio",
			wantValue:        false,
		},
		{
			name:             "shot_type_with_preferResolution_false",
			graph:            optionGraph(map[string]any{"shot_type": "close_up"}),
			preferResolution: false,
			wantKey:          "shot_type",
			wantValue:        "close_up",
		},
		{
			name:             "prompt_extend_present",
			graph:            optionGraph(map[string]any{"prompt_extend": true}),
			preferResolution: false,
			wantKey:          "prompt_extend",
			wantValue:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := BuildParameters(tt.graph, tt.preferResolution)
			got, ok := p[tt.wantKey]
			if !ok {
				t.Fatalf("BuildParameters() missing key %q; got %v", tt.wantKey, p)
			}
			if got != tt.wantValue {
				t.Errorf("BuildParameters()[%q] = %v, want %v", tt.wantKey, got, tt.wantValue)
			}
		})
	}
}

func TestBuildParameters_Defaults(t *testing.T) {
	t.Parallel()

	t.Run("default_size_when_preferResolution_false", func(t *testing.T) {
		t.Parallel()
		g := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}}}
		p := BuildParameters(g, false)
		if p["size"] != "1280*720" {
			t.Errorf("expected default size 1280*720, got %v", p["size"])
		}
	})

	t.Run("default_resolution_when_preferResolution_true", func(t *testing.T) {
		t.Parallel()
		g := workflow.Graph{"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}}}
		p := BuildParameters(g, true)
		if p["resolution"] != "720P" {
			t.Errorf("expected default resolution 720P, got %v", p["resolution"])
		}
	})
}

func TestBuildParameters_AudioOmittedWhenPreferResolution(t *testing.T) {
	t.Parallel()
	g := optionGraph(map[string]any{"audio": false, "resolution": "1080P"})
	p := BuildParameters(g, true)
	if _, has := p["audio"]; has {
		t.Error("audio should not be present when preferResolution=true")
	}
}

func TestBuildParameters_ShotTypeOmittedWhenPreferResolution(t *testing.T) {
	t.Parallel()
	g := optionGraph(map[string]any{"shot_type": "close_up", "resolution": "1080P"})
	p := BuildParameters(g, true)
	if _, has := p["shot_type"]; has {
		t.Error("shot_type should not be present when preferResolution=true")
	}
}

func TestBuildParameters_WidthHeightSize(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
	}
	p := BuildParameters(g, false)
	if p["size"] != "1920*1080" {
		t.Errorf("expected size 1920*1080 from width/height, got %v", p["size"])
	}
}
