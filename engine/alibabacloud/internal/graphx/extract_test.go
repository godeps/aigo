package graphx

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestImageURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"single image",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			[]string{"http://a.png"},
		},
		{
			"video node ignored",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			nil,
		},
		{
			"mixed nodes",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
				"3": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://b.png"}},
			},
			[]string{"http://a.png", "http://b.png"},
		},
		{
			"empty url ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": ""}},
			},
			nil,
		},
		{
			"no url input ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"text": "hello"}},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ImageURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("ImageURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"single video",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			[]string{"http://v.mp4"},
		},
		{
			"image node ignored",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			nil,
		},
		{
			"mixed nodes",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v1.mp4"}},
				"3": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v2.mp4"}},
			},
			[]string{"http://v1.mp4", "http://v2.mp4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VideoURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("VideoURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaURLs(t *testing.T) {
	tests := []struct {
		name string
		graph workflow.Graph
		want []string
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"collects both image and video",
			workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
				"2": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
			},
			[]string{"http://a.png", "http://v.mp4"},
		},
		{
			"ignores non-media nodes",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"url": "http://x.png"}},
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MediaURLs(tt.graph)
			if !strSliceEqual(got, tt.want) {
				t.Errorf("MediaURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVideoEditMedia(t *testing.T) {
	tests := []struct {
		name  string
		graph workflow.Graph
		want  []map[string]any
	}{
		{"empty graph", workflow.Graph{}, nil},
		{
			"video and image",
			workflow.Graph{
				"1": {ClassType: "LoadVideo", Inputs: map[string]any{"url": "http://v.mp4"}},
				"2": {ClassType: "LoadImage", Inputs: map[string]any{"url": "http://a.png"}},
			},
			[]map[string]any{
				{"type": "video", "url": "http://v.mp4"},
				{"type": "reference_image", "url": "http://a.png"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VideoEditMedia(tt.graph)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("VideoEditMedia() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i]["type"] != tt.want[i]["type"] || got[i]["url"] != tt.want[i]["url"] {
					t.Errorf("VideoEditMedia()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPrompt(t *testing.T) {
	tests := []struct {
		name    string
		graph   workflow.Graph
		want    string
		wantErr bool
	}{
		{
			"from CLIPTextEncode",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a cat"}},
			},
			"a cat", false,
		},
		{
			"from option fallback",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"prompt": "a dog"}},
			},
			"a dog", false,
		},
		{
			"missing prompt",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 512}},
			},
			"", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Prompt(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Prompt() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Prompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeSize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"1024x1024", "1024*1024"},
		{"1280*720", "1280*720"},
		{"512x512", "512*512"},
	}
	for _, tt := range tests {
		if got := NormalizeSize(tt.input); got != tt.want {
			t.Errorf("NormalizeSize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSize(t *testing.T) {
	tests := []struct {
		name     string
		graph    workflow.Graph
		fallback string
		want     string
	}{
		{
			"from option",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280x720"}},
			},
			"default",
			"1280*720",
		},
		{
			"from EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"default",
			"1920*1080",
		},
		{
			"fallback",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"default",
			"default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Size(tt.graph, tt.fallback); got != tt.want {
				t.Errorf("Size() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeriveResolution(t *testing.T) {
	tests := []struct {
		name  string
		graph workflow.Graph
		want  string
		ok    bool
	}{
		{
			"from size 1280*720",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280*720"}},
			},
			"720P", true,
		},
		{
			"from size 1920*1080",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1920*1080"}},
			},
			"1080P", true,
		},
		{
			"from EmptyLatentImage 1920x1080",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"1080P", true,
		},
		{
			"no resolution available",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DeriveResolution(tt.graph)
			if ok != tt.ok || got != tt.want {
				t.Errorf("DeriveResolution() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// strSliceEqual compares two string slices, treating nil and empty as equal.
func strSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestResolution(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"explicit resolution in Options",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"resolution": "4K"}},
			},
			"4K", true,
		},
		{
			"derived from size 1280*720",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1280*720"}},
			},
			"720P", true,
		},
		{
			"derived from size 1920*1080",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1920*1080"}},
			},
			"1080P", true,
		},
		{
			"derived from EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"1080P", true,
		},
		{
			"no resolution at all",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Resolution(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("Resolution() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAudioVoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from AudioOptions node",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "alloy"}},
			},
			"alloy", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"voice": "echo"}},
			},
			"echo", true,
		},
		{
			"AudioOptions takes priority over Options",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "alloy"}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{"voice": "echo"}},
			},
			"alloy", true,
		},
		{
			"missing voice",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
		{
			"empty voice ignored",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "  "}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := AudioVoice(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("AudioVoice() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAudioLanguageType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from AudioOptions node",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"language_type": "zh"}},
			},
			"zh", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"language_type": "en"}},
			},
			"en", true,
		},
		{
			"missing language_type",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := AudioLanguageType(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("AudioLanguageType() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAudioInstructions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from AudioOptions node",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"instructions": "speak slowly"}},
			},
			"speak slowly", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"instructions": "speak fast"}},
			},
			"speak fast", true,
		},
		{
			"AudioOptions takes priority",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"instructions": "primary"}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{"instructions": "fallback"}},
			},
			"primary", true,
		},
		{
			"missing instructions",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := AudioInstructions(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("AudioInstructions() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAudioOptimizeInstructions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   bool
		wantOK bool
	}{
		{
			"bool true",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"optimize_instructions": true}},
			},
			true, true,
		},
		{
			"bool false",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"optimize_instructions": false}},
			},
			false, true,
		},
		{
			"string true",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"optimize_instructions": "true"}},
			},
			true, true,
		},
		{
			"string false",
			workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"optimize_instructions": "false"}},
			},
			false, true,
		},
		{
			"fallback to BoolOption",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"optimize_instructions": true}},
			},
			true, true,
		},
		{
			"missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			false, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := AudioOptimizeInstructions(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("AudioOptimizeInstructions() = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestVoiceDesignOmitPreview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		graph workflow.Graph
		want  bool
	}{
		{
			"VoiceDesignInput bool true",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"omit_preview": true}},
			},
			true,
		},
		{
			"VoiceDesignInput bool false",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"omit_preview": false}},
			},
			false,
		},
		{
			"VoiceDesignInput string true",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"omit_preview": "true"}},
			},
			true,
		},
		{
			"Options fallback bool true",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"omit_preview": true}},
			},
			true,
		},
		{
			"Options fallback string true",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"omit_preview": "true"}},
			},
			true,
		},
		{
			"missing defaults false",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := VoiceDesignOmitPreview(tt.graph)
			if got != tt.want {
				t.Errorf("VoiceDesignOmitPreview() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVoiceDesignFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		graph           workflow.Graph
		wantVoicePrompt string
		wantPreviewText string
		wantTargetModel string
		wantErr         bool
	}{
		{
			"all from VoiceDesignInput",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
					"voice_prompt": "warm female",
					"preview_text": "hello world",
					"target_model": "cosyvoice-v2",
				}},
			},
			"warm female", "hello world", "cosyvoice-v2", false,
		},
		{
			"all from Options fallback",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{
					"voice_prompt": "deep male",
					"preview_text": "test text",
					"target_model": "cosyvoice-v1",
				}},
			},
			"deep male", "test text", "cosyvoice-v1", false,
		},
		{
			"mixed VoiceDesignInput and Options",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
					"voice_prompt": "warm female",
				}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{
					"preview_text": "hello",
					"target_model": "cosyvoice-v2",
				}},
			},
			"warm female", "hello", "cosyvoice-v2", false,
		},
		{
			"missing voice_prompt",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
					"preview_text": "hello",
					"target_model": "cosyvoice-v2",
				}},
			},
			"", "", "", true,
		},
		{
			"missing preview_text",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
					"voice_prompt": "warm female",
					"target_model": "cosyvoice-v2",
				}},
			},
			"", "", "", true,
		},
		{
			"missing target_model",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{
					"voice_prompt": "warm female",
					"preview_text": "hello",
				}},
			},
			"", "", "", true,
		},
		{
			"all missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", "", "", true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			vp, pt, tm, err := VoiceDesignFields(tt.graph)
			if (err != nil) != tt.wantErr {
				t.Fatalf("VoiceDesignFields() error = %v, wantErr %v", err, tt.wantErr)
			}
			if vp != tt.wantVoicePrompt || pt != tt.wantPreviewText || tm != tt.wantTargetModel {
				t.Errorf("VoiceDesignFields() = (%q, %q, %q), want (%q, %q, %q)",
					vp, pt, tm, tt.wantVoicePrompt, tt.wantPreviewText, tt.wantTargetModel)
			}
		})
	}
}

func TestVoiceDesignPreferredName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from VoiceDesignInput",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"preferred_name": "Alice"}},
			},
			"Alice", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"preferred_name": "Bob"}},
			},
			"Bob", true,
		},
		{
			"VoiceDesignInput takes priority",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"preferred_name": "Alice"}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{"preferred_name": "Bob"}},
			},
			"Alice", true,
		},
		{
			"missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := VoiceDesignPreferredName(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("VoiceDesignPreferredName() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestVoiceDesignLanguage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from VoiceDesignInput",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"language": "zh"}},
			},
			"zh", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"language": "en"}},
			},
			"en", true,
		},
		{
			"missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := VoiceDesignLanguage(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("VoiceDesignLanguage() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestVoiceDesignSampleRate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   int
		wantOK bool
	}{
		{
			"from VoiceDesignInput",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"sample_rate": 44100}},
			},
			44100, true,
		},
		{
			"fallback to IntOption",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"sample_rate": 22050}},
			},
			22050, true,
		},
		{
			"VoiceDesignInput takes priority",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"sample_rate": 44100}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{"sample_rate": 22050}},
			},
			44100, true,
		},
		{
			"missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			0, false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := VoiceDesignSampleRate(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("VoiceDesignSampleRate() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestVoiceDesignResponseFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		graph  workflow.Graph
		want   string
		wantOK bool
	}{
		{
			"from VoiceDesignInput",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"response_format": "wav"}},
			},
			"wav", true,
		},
		{
			"fallback to Options",
			workflow.Graph{
				"1": {ClassType: "SomeOptions", Inputs: map[string]any{"response_format": "mp3"}},
			},
			"mp3", true,
		},
		{
			"VoiceDesignInput takes priority",
			workflow.Graph{
				"1": {ClassType: "VoiceDesignInput", Inputs: map[string]any{"response_format": "wav"}},
				"2": {ClassType: "SomeOptions", Inputs: map[string]any{"response_format": "mp3"}},
			},
			"wav", true,
		},
		{
			"missing",
			workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hi"}},
			},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := VoiceDesignResponseFormat(tt.graph)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("VoiceDesignResponseFormat() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
