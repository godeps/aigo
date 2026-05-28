package vidgen

import "testing"

func TestIsTextToVideoModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"wan-ai/wan2.1-t2v-plus", true},
		{"wan2.1-t2v-turbo", true},
		{"some-model-t2v", true},
		{"prefix-t2v-suffix", true},
		{"wan-ai/wan2.1-i2v-plus", false},
		{"wan-ai/wan2.1-r2v-plus", false},
		{"kling-video-generation", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			if got := IsTextToVideoModel(tt.model); got != tt.want {
				t.Errorf("IsTextToVideoModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsReferenceToVideoModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"wan-ai/wan2.1-r2v-plus", true},
		{"wan-ai/wan2.1-i2v-plus", true},
		{"model-r2v", true},
		{"model-i2v", true},
		{"model-r2v-i2v", true},
		{"wan-ai/wan2.1-t2v-plus", false},
		{"kling-video-generation", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			if got := IsReferenceToVideoModel(tt.model); got != tt.want {
				t.Errorf("IsReferenceToVideoModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsKlingVideoModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"kling/kling-v3-video-generation", true},
		{"kling/kling-v3-omni-video-generation", true},
		{"some-kling-video-generation-model", true},
		{"kling-only", false},
		{"video-generation-only", false},
		{"wan-ai/wan2.1-t2v-plus", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			if got := IsKlingVideoModel(tt.model); got != tt.want {
				t.Errorf("IsKlingVideoModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsVideoEditModel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model string
		want  bool
	}{
		{"wan-ai/wan2.1-videoedit-plus", true},
		{"some-videoedit-model", true},
		{"prefix-videoedit", true},
		{"wan-ai/wan2.1-t2v-plus", false},
		{"wan-ai/wan2.1-i2v-plus", false},
		{"kling-video-generation", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			t.Parallel()
			if got := IsVideoEditModel(tt.model); got != tt.want {
				t.Errorf("IsVideoEditModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}
