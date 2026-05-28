package graph

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestExtractImageSizePrefersImageOptions(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1536*1024"}},
		"2": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "x"}},
		"3": {ClassType: "NegativePrompt", Inputs: map[string]any{"size": "256x256"}},
	}
	got := ExtractImageSizeOpenAI(g)
	if got != "1536x1024" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractNegativePromptPrefersNode(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "NegativePrompt", Inputs: map[string]any{"negative_prompt": "from node"}},
		"2": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "p", "negative_prompt": "wrong"}},
	}
	v, ok := ExtractNegativePrompt(g)
	if !ok || v != "from node" {
		t.Fatalf("got %q %v", v, ok)
	}
}

func TestExtractPrompt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		g       workflow.Graph
		want    string
		wantErr error
	}{
		{
			name: "CLIPTextEncode node with text",
			g: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
			},
			want: "a beautiful sunset",
		},
		{
			name: "flat prompt key",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello world"}},
			},
			want: "hello world",
		},
		{
			name: "flat text key",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"text": "from text key"}},
			},
			want: "from text key",
		},
		{
			name: "flat value key",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"value": "from value key"}},
			},
			want: "from value key",
		},
		{
			name:    "empty graph returns ErrMissingPrompt",
			g:       workflow.Graph{},
			wantErr: ErrMissingPrompt,
		},
		{
			name: "blank prompt returns ErrMissingPrompt",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "   "}},
			},
			wantErr: ErrMissingPrompt,
		},
		{
			name: "CLIPTextEncode preferred over flat key",
			g: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "from clip"}},
				"2": {ClassType: "Options", Inputs: map[string]any{"prompt": "from flat"}},
			},
			want: "from clip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExtractPrompt(tt.g)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Fatalf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstImageURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		want   string
		wantOK bool
	}{
		{
			name: "LoadImage node with url",
			g: workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://example.com/img.png"}},
			},
			want:   "https://example.com/img.png",
			wantOK: true,
		},
		{
			name: "image class with url",
			g: workflow.Graph{
				"1": {ClassType: "UploadImage", Inputs: map[string]any{"url": "https://example.com/upload.jpg"}},
			},
			want:   "https://example.com/upload.jpg",
			wantOK: true,
		},
		{
			name: "no images returns false",
			g: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
			},
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty graph returns false",
			g:      workflow.Graph{},
			want:   "",
			wantOK: false,
		},
		{
			name: "LoadImage preferred over other image nodes",
			g: workflow.Graph{
				"1": {ClassType: "LoadImage", Inputs: map[string]any{"url": "https://first.com/a.png"}},
				"2": {ClassType: "UploadImage", Inputs: map[string]any{"url": "https://second.com/b.png"}},
			},
			want:   "https://first.com/a.png",
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := FirstImageURL(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractVideoDimensions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		g                      workflow.Graph
		wantW, wantH           int
		wantOK                 bool
	}{
		{
			name: "VideoOptions with width and height",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			wantW: 1920, wantH: 1080, wantOK: true,
		},
		{
			name: "flat options with width and height",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"width": 1280, "height": 720}},
			},
			wantW: 1280, wantH: 720, wantOK: true,
		},
		{
			name: "EmptyLatentImage fallback",
			g: workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 512, "height": 512}},
			},
			wantW: 512, wantH: 512, wantOK: true,
		},
		{
			name: "no dimensions",
			g: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "hello"}},
			},
			wantW: 0, wantH: 0, wantOK: false,
		},
		{
			name:   "empty graph",
			g:      workflow.Graph{},
			wantW:  0, wantH: 0, wantOK: false,
		},
		{
			name: "VideoOptions preferred over flat",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"width": 640, "height": 480}},
				"2": {ClassType: "VideoOptions", Inputs: map[string]any{"width": 3840, "height": 2160}},
			},
			wantW: 3840, wantH: 2160, wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h, ok := ExtractVideoDimensions(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if w != tt.wantW || h != tt.wantH {
				t.Fatalf("got %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestExtractSpeechVoice(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		want   string
		wantOK bool
	}{
		{
			name: "from AudioOptions",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "alloy"}},
			},
			want:   "alloy",
			wantOK: true,
		},
		{
			name: "from flat options",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"voice": "nova"}},
			},
			want:   "nova",
			wantOK: true,
		},
		{
			name: "missing voice",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			want:   "",
			wantOK: false,
		},
		{
			name: "AudioOptions preferred over flat",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "echo"}},
				"2": {ClassType: "Options", Inputs: map[string]any{"voice": "onyx"}},
			},
			want:   "echo",
			wantOK: true,
		},
		{
			name: "blank voice skipped",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"voice": "   "}},
				"2": {ClassType: "Options", Inputs: map[string]any{"voice": "shimmer"}},
			},
			want:   "shimmer",
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractSpeechVoice(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSpeechResponseFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		g    workflow.Graph
		want string
	}{
		{
			name: "from AudioOptions",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"response_format": "wav"}},
			},
			want: "wav",
		},
		{
			name: "from flat options",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"response_format": "flac"}},
			},
			want: "flac",
		},
		{
			name: "default mp3",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			want: "mp3",
		},
		{
			name: "empty graph defaults to mp3",
			g:    workflow.Graph{},
			want: "mp3",
		},
		{
			name: "AudioOptions preferred over flat",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"response_format": "opus"}},
				"2": {ClassType: "Options", Inputs: map[string]any{"response_format": "aac"}},
			},
			want: "opus",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractSpeechResponseFormat(tt.g)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractSpeechSpeed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		want   float64
		wantOK bool
	}{
		{
			name: "from AudioOptions float64",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"speed": 1.5}},
			},
			want:   1.5,
			wantOK: true,
		},
		{
			name: "from AudioOptions int",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"speed": 2}},
			},
			want:   2.0,
			wantOK: true,
		},
		{
			name: "from AudioOptions string",
			g: workflow.Graph{
				"1": {ClassType: "AudioOptions", Inputs: map[string]any{"speed": "0.75"}},
			},
			want:   0.75,
			wantOK: true,
		},
		{
			name: "from flat options int",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"speed": 2}},
			},
			want:   2.0,
			wantOK: true,
		},
		{
			name: "missing returns false",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			want:   0,
			wantOK: false,
		},
		{
			name:   "empty graph",
			g:      workflow.Graph{},
			want:   0,
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractSpeechSpeed(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestExtractVideoDuration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		want   float64
		wantOK bool
	}{
		{
			name: "from VideoOptions float64",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5.0}},
			},
			want:   5.0,
			wantOK: true,
		},
		{
			name: "from VideoOptions int",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 10}},
			},
			want:   10.0,
			wantOK: true,
		},
		{
			name: "from flat options float64",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"duration": 3.0}},
			},
			want:   3.0,
			wantOK: true,
		},
		{
			name: "from flat options int",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"duration": 7}},
			},
			want:   7.0,
			wantOK: true,
		},
		{
			name: "missing returns false",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			want:   0,
			wantOK: false,
		},
		{
			name:   "empty graph",
			g:      workflow.Graph{},
			want:   0,
			wantOK: false,
		},
		{
			name: "zero duration treated as missing",
			g: workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 0.0}},
			},
			want:   0,
			wantOK: false,
		},
		{
			name: "VideoOptions preferred over flat",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"duration": 2.0}},
				"2": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 8.0}},
			},
			want:   8.0,
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractVideoDuration(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %f, want %f", got, tt.want)
			}
		})
	}
}

func TestExtractImageSizeOpenAI_Fallbacks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		g    workflow.Graph
		want string
	}{
		{
			name: "flat size option with asterisk",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"size": "512*512"}},
			},
			want: "512x512",
		},
		{
			name: "EmptyLatentImage fallback 1024x1536",
			g: workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1536}},
			},
			want: "1024x1536",
		},
		{
			name:   "empty graph defaults to 1024x1024",
			g:      workflow.Graph{},
			want:   "1024x1024",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractImageSizeOpenAI(tt.g)
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractNegativePrompt_Fallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		g      workflow.Graph
		want   string
		wantOK bool
	}{
		{
			name: "flat option fallback",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"negative_prompt": "ugly, blurry"}},
			},
			want:   "ugly, blurry",
			wantOK: true,
		},
		{
			name: "missing returns false",
			g: workflow.Graph{
				"1": {ClassType: "Options", Inputs: map[string]any{"prompt": "hello"}},
			},
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty graph",
			g:      workflow.Graph{},
			want:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractNegativePrompt(tt.g)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
