package resolve

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestParseSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		input       string
		wantW, wantH int
		wantAR      string
		wantRes     string
	}{
		{"WxH lowercase", "1024x1024", 1024, 1024, "1:1", "720P"},
		{"WxH uppercase", "1920X1080", 1920, 1080, "16:9", "1080P"},
		{"W*H asterisk", "1280*720", 1280, 720, "16:9", "720P"},
		{"WxH portrait", "1024x1536", 1024, 1536, "2:3", "720P"},
		{"WxH landscape", "1536x1024", 1536, 1024, "3:2", "720P"},
		{"aspect ratio 16:9", "16:9", 0, 0, "16:9", ""},
		{"aspect ratio 1:1", "1:1", 0, 0, "1:1", ""},
		{"aspect ratio 9:16", "9:16", 0, 0, "9:16", ""},
		{"resolution 720P", "720P", 0, 0, "", "720P"},
		{"resolution 1080p", "1080p", 0, 0, "", "1080P"},
		{"resolution 480P", "480P", 0, 0, "", "480P"},
		{"empty string", "", 0, 0, "", ""},
		{"whitespace", "  ", 0, 0, "", ""},
		{"invalid", "hello", 0, 0, "", ""},
		{"WxH with spaces", " 1024 x 1024 ", 1024, 1024, "1:1", "720P"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ParseSize(tc.input)

			if tc.wantW == 0 && tc.wantH == 0 {
				if spec.Dimensions != nil {
					t.Errorf("expected nil Dimensions, got %+v", spec.Dimensions)
				}
			} else {
				if spec.Dimensions == nil {
					t.Fatalf("expected Dimensions{%d,%d}, got nil", tc.wantW, tc.wantH)
				}
				if spec.Dimensions.Width != tc.wantW || spec.Dimensions.Height != tc.wantH {
					t.Errorf("Dimensions = {%d,%d}, want {%d,%d}",
						spec.Dimensions.Width, spec.Dimensions.Height, tc.wantW, tc.wantH)
				}
			}
			if spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
			if spec.Resolution != tc.wantRes {
				t.Errorf("Resolution = %q, want %q", spec.Resolution, tc.wantRes)
			}
		})
	}
}

func TestParseDimensions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		w, h    int
		wantAR  string
		wantRes string
	}{
		{"square 1024", 1024, 1024, "1:1", "720P"},
		{"16:9 HD", 1280, 720, "16:9", "720P"},
		{"16:9 FHD", 1920, 1080, "16:9", "1080P"},
		{"9:16 portrait", 720, 1280, "9:16", "720P"},
		{"9:16 FHD portrait", 1080, 1920, "9:16", "1080P"},
		{"4:3", 1024, 768, "4:3", "720P"},
		{"3:4", 768, 1024, "3:4", "720P"},
		{"zero width", 0, 1024, "", ""},
		{"negative", -1, 100, "", ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ParseDimensions(tc.w, tc.h)
			if tc.wantAR == "" {
				if !spec.IsZero() {
					t.Errorf("expected zero spec, got %+v", spec)
				}
				return
			}
			if spec.Dimensions == nil {
				t.Fatal("expected non-nil Dimensions")
			}
			if spec.Dimensions.Width != tc.w || spec.Dimensions.Height != tc.h {
				t.Errorf("Dimensions = {%d,%d}, want {%d,%d}",
					spec.Dimensions.Width, spec.Dimensions.Height, tc.w, tc.h)
			}
			if spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
			if spec.Resolution != tc.wantRes {
				t.Errorf("Resolution = %q, want %q", spec.Resolution, tc.wantRes)
			}
		})
	}
}

func TestSizeSpec_ToWxH(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec SizeSpec
		want string
	}{
		{"with dimensions", SizeSpec{Dimensions: &Dimensions{1024, 1024}}, "1024x1024"},
		{"nil dimensions", SizeSpec{AspectRatio: "16:9"}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.ToWxH(); got != tc.want {
				t.Errorf("ToWxH() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSizeSpec_ToWAsteriskH(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec SizeSpec
		want string
	}{
		{"with dimensions", SizeSpec{Dimensions: &Dimensions{1280, 720}}, "1280*720"},
		{"nil dimensions", SizeSpec{AspectRatio: "16:9"}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.ToWAsteriskH(); got != tc.want {
				t.Errorf("ToWAsteriskH() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSizeSpec_ToAspectRatio(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec SizeSpec
		want string
	}{
		{"explicit ratio", SizeSpec{AspectRatio: "4:3"}, "4:3"},
		{"from dimensions 16:9", SizeSpec{Dimensions: &Dimensions{1920, 1080}}, "16:9"},
		{"from dimensions 1:1", SizeSpec{Dimensions: &Dimensions{512, 512}}, "1:1"},
		{"empty", SizeSpec{}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.ToAspectRatio(); got != tc.want {
				t.Errorf("ToAspectRatio() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSizeSpec_ToResolution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		spec SizeSpec
		want string
	}{
		{"explicit resolution", SizeSpec{Resolution: "1080P"}, "1080P"},
		{"from FHD dimensions", SizeSpec{Dimensions: &Dimensions{1920, 1080}}, "1080P"},
		{"from HD dimensions", SizeSpec{Dimensions: &Dimensions{1280, 720}}, "720P"},
		{"from small dimensions", SizeSpec{Dimensions: &Dimensions{320, 240}}, ""},
		{"empty", SizeSpec{}, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.spec.ToResolution(); got != tc.want {
				t.Errorf("ToResolution() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSizeSpec_SnapTo(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		input     SizeSpec
		supported []string
		wantW     int
		wantH     int
	}{
		{
			"landscape to nearest landscape",
			ParseDimensions(1920, 1080),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1536, 1024,
		},
		{
			"portrait to nearest portrait",
			ParseDimensions(720, 1280),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1024, 1536,
		},
		{
			"square to square",
			ParseDimensions(512, 512),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1024, 1024,
		},
		{
			"video landscape snap",
			ParseDimensions(1920, 1080),
			[]string{"1280x720", "960x960", "720x1280", "1920x1080"},
			1920, 1080,
		},
		{
			"video portrait snap",
			ParseDimensions(1080, 1920),
			[]string{"1280x720", "960x960", "720x1280", "1920x1080", "1080x1920"},
			1080, 1920,
		},
		{
			"aspect ratio only - 16:9 to landscape",
			ParseSize("16:9"),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1536, 1024,
		},
		{
			"aspect ratio only - 9:16 to portrait",
			ParseSize("9:16"),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1024, 1536,
		},
		{
			"aspect ratio only - 1:1 to square",
			ParseSize("1:1"),
			[]string{"1024x1024", "1536x1024", "1024x1536"},
			1024, 1024,
		},
		{
			"empty supported returns original",
			ParseDimensions(1920, 1080),
			nil,
			1920, 1080,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := tc.input.SnapTo(tc.supported)
			if tc.supported == nil {
				// Should return original
				if result.Dimensions == nil {
					t.Fatal("expected non-nil Dimensions for nil supported")
				}
				if result.Dimensions.Width != tc.wantW || result.Dimensions.Height != tc.wantH {
					t.Errorf("got {%d,%d}, want {%d,%d}",
						result.Dimensions.Width, result.Dimensions.Height, tc.wantW, tc.wantH)
				}
				return
			}
			if result.Dimensions == nil {
				t.Fatal("expected non-nil Dimensions after SnapTo")
			}
			if result.Dimensions.Width != tc.wantW || result.Dimensions.Height != tc.wantH {
				t.Errorf("got {%d,%d}, want {%d,%d}",
					result.Dimensions.Width, result.Dimensions.Height, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestNormalizeImageSize(t *testing.T) {
	t.Parallel()

	openAISizes := []string{"1024x1024", "1024x1536", "1536x1024"}

	cases := []struct {
		name string
		w, h int
		want string
	}{
		{"exact square", 1024, 1024, "1024x1024"},
		{"exact portrait", 1024, 1536, "1024x1536"},
		{"exact landscape", 1536, 1024, "1536x1024"},
		{"landscape snaps", 1920, 1080, "1536x1024"},
		{"portrait snaps", 1080, 1920, "1024x1536"},
		{"small square snaps", 512, 512, "1024x1024"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeImageSize(openAISizes, tc.w, tc.h)
			if got != tc.want {
				t.Errorf("NormalizeImageSize(%d, %d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestNormalizeOpenAIImageSize_BackwardsCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		w, h int
		want string
	}{
		{"1024x1024", 1024, 1024, "1024x1024"},
		{"1024x1536", 1024, 1536, "1024x1536"},
		{"1536x1024", 1536, 1024, "1536x1024"},
		{"landscape", 1920, 1080, "1536x1024"},
		{"portrait", 720, 1280, "1024x1536"},
		{"square default", 800, 800, "1024x1024"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeOpenAIImageSize(tc.w, tc.h)
			if got != tc.want {
				t.Errorf("NormalizeOpenAIImageSize(%d, %d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestNormalizeVideoSize(t *testing.T) {
	t.Parallel()

	supported := []string{"1280x720", "960x960", "720x1280", "1920x1080", "1080x1920"}

	cases := []struct {
		name       string
		w, h       int
		wantW      int
		wantH      int
	}{
		{"exact match", 1280, 720, 1280, 720},
		{"FHD landscape", 1920, 1080, 1920, 1080},
		{"portrait", 1080, 1920, 1080, 1920},
		{"square", 1024, 1024, 960, 960},
		{"near landscape", 1600, 900, 1280, 720},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := NormalizeVideoSize(supported, tc.w, tc.h)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("NormalizeVideoSize(%d, %d) = (%d, %d), want (%d, %d)",
					tc.w, tc.h, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestExtractImageSizeSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		graph workflow.Graph
		wantW int
		wantH int
		wantAR string
	}{
		{
			"from ImageOptions size",
			workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1024x1536"}},
			},
			1024, 1536, "2:3",
		},
		{
			"from ImageOptions aspect_ratio",
			workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"aspect_ratio": "16:9"}},
			},
			0, 0, "16:9",
		},
		{
			"from global size option",
			workflow.Graph{
				"1": {ClassType: "SomeNode", Inputs: map[string]any{"size": "1920x1080"}},
			},
			1920, 1080, "16:9",
		},
		{
			"from EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 1024, "height": 1024}},
			},
			1024, 1024, "1:1",
		},
		{
			"ImageOptions takes priority over EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1536x1024"}},
				"2": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 512, "height": 512}},
			},
			1536, 1024, "3:2",
		},
		{
			"empty graph",
			workflow.Graph{},
			0, 0, "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ExtractImageSizeSpec(tc.graph)
			if tc.wantW == 0 && tc.wantH == 0 {
				if spec.Dimensions != nil {
					t.Errorf("expected nil Dimensions, got %+v", spec.Dimensions)
				}
			} else {
				if spec.Dimensions == nil {
					t.Fatalf("expected Dimensions{%d,%d}, got nil", tc.wantW, tc.wantH)
				}
				if spec.Dimensions.Width != tc.wantW || spec.Dimensions.Height != tc.wantH {
					t.Errorf("Dimensions = {%d,%d}, want {%d,%d}",
						spec.Dimensions.Width, spec.Dimensions.Height, tc.wantW, tc.wantH)
				}
			}
			if tc.wantAR != "" && spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
		})
	}
}

func TestExtractVideoSizeSpec(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		graph  workflow.Graph
		wantW  int
		wantH  int
		wantAR string
		wantRes string
	}{
		{
			"from VideoOptions width/height",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"width": 1280, "height": 720}},
			},
			1280, 720, "16:9", "720P",
		},
		{
			"from VideoOptions size string",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"size": "1920x1080", "aspect_ratio": "16:9"}},
			},
			1920, 1080, "16:9", "1080P",
		},
		{
			"from VideoOptions aspect_ratio + resolution",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"aspect_ratio": "9:16", "resolution": "720P"}},
			},
			0, 0, "9:16", "720P",
		},
		{
			"from global options",
			workflow.Graph{
				"1": {ClassType: "SomeNode", Inputs: map[string]any{"size": "1280x720"}},
			},
			1280, 720, "16:9", "720P",
		},
		{
			"from EmptyLatentImage",
			workflow.Graph{
				"1": {ClassType: "EmptyLatentImage", Inputs: map[string]any{"width": 960, "height": 960}},
			},
			960, 960, "1:1", "720P",
		},
		{
			"empty graph",
			workflow.Graph{},
			0, 0, "", "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ExtractVideoSizeSpec(tc.graph)
			if tc.wantW == 0 && tc.wantH == 0 {
				if spec.Dimensions != nil {
					t.Errorf("expected nil Dimensions, got %+v", spec.Dimensions)
				}
			} else {
				if spec.Dimensions == nil {
					t.Fatalf("expected Dimensions{%d,%d}, got nil", tc.wantW, tc.wantH)
				}
				if spec.Dimensions.Width != tc.wantW || spec.Dimensions.Height != tc.wantH {
					t.Errorf("Dimensions = {%d,%d}, want {%d,%d}",
						spec.Dimensions.Width, spec.Dimensions.Height, tc.wantW, tc.wantH)
				}
			}
			if tc.wantAR != "" && spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
			if tc.wantRes != "" && spec.Resolution != tc.wantRes {
				t.Errorf("Resolution = %q, want %q", spec.Resolution, tc.wantRes)
			}
		})
	}
}

func TestSimplifyRatio(t *testing.T) {
	t.Parallel()

	cases := []struct {
		w, h int
		want string
	}{
		{1920, 1080, "16:9"},
		{1080, 1920, "9:16"},
		{1024, 1024, "1:1"},
		{1024, 768, "4:3"},
		{768, 1024, "3:4"},
		{1280, 720, "16:9"},
		{1536, 1024, "3:2"},
		{1024, 1536, "2:3"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := simplifyRatio(tc.w, tc.h)
			if got != tc.want {
				t.Errorf("simplifyRatio(%d, %d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestDeriveResolution(t *testing.T) {
	t.Parallel()

	cases := []struct {
		w, h int
		want string
	}{
		{3840, 2160, "4K"},
		{2560, 1440, "2K"},
		{1920, 1080, "1080P"},
		{1280, 720, "720P"},
		{640, 480, "480P"},
		{320, 240, ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			got := deriveResolution(tc.w, tc.h)
			if got != tc.want {
				t.Errorf("deriveResolution(%d, %d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestSizeSpec_ToWidthHeight(t *testing.T) {
	t.Parallel()
	w, h := SizeSpec{Dimensions: &Dimensions{1920, 1080}}.ToWidthHeight()
	if w != 1920 || h != 1080 {
		t.Errorf("got (%d,%d), want (1920,1080)", w, h)
	}
	w, h = SizeSpec{AspectRatio: "16:9"}.ToWidthHeight()
	if w != 0 || h != 0 {
		t.Errorf("got (%d,%d), want (0,0)", w, h)
	}
}

func TestNormalizeImageSize_EdgeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		supported []string
		w, h      int
		want      string
	}{
		{"nil supported with valid dims", nil, 1920, 1080, "1920x1080"},
		{"empty supported uses first fallback", []string{"512x512"}, 0, 0, "512x512"},
		{"zero dims returns first supported", []string{"768x768", "1024x1024"}, 0, 0, "768x768"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeImageSize(tc.supported, tc.w, tc.h)
			if got != tc.want {
				t.Errorf("NormalizeImageSize(%v, %d, %d) = %q, want %q", tc.supported, tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestNormalizeVideoSize_EdgeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		supported []string
		w, h      int
		wantW     int
		wantH     int
	}{
		{"nil supported returns original", nil, 1600, 900, 1600, 900},
		{"zero dims with supported returns first", []string{"1280x720"}, 0, 0, 1280, 720},
		{"zero dims nil supported returns zeros", nil, 0, 0, 0, 0},
		{"supported has only ratios falls through", []string{"16:9"}, 1920, 1080, 1920, 1080},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := NormalizeVideoSize(tc.supported, tc.w, tc.h)
			if gotW != tc.wantW || gotH != tc.wantH {
				t.Errorf("NormalizeVideoSize(%v, %d, %d) = (%d,%d), want (%d,%d)",
					tc.supported, tc.w, tc.h, gotW, gotH, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestSimplifyRatio_Approximations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		w, h int
		want string
	}{
		{"near 16:9", 1778, 1000, "16:9"},
		{"near 9:16", 1000, 1778, "9:16"},
		{"near 4:3", 1333, 1000, "4:3"},
		{"near 3:4", 1000, 1333, "3:4"},
		{"near 1:1", 1001, 1000, "1:1"},
		{"near 3:2", 1500, 1000, "3:2"},
		{"near 2:3", 1000, 1500, "2:3"},
		{"exact odd ratio", 700, 300, "7:3"},
		{"zero width", 0, 100, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := simplifyRatio(tc.w, tc.h)
			if got != tc.want {
				t.Errorf("simplifyRatio(%d, %d) = %q, want %q", tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestExtractImageSizeSpec_AdditionalPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		graph workflow.Graph
		wantAR string
	}{
		{
			"global aspect_ratio only",
			workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"aspect_ratio": "4:3"}},
			},
			"4:3",
		},
		{
			"global width/height integers",
			workflow.Graph{
				"1": {ClassType: "Params", Inputs: map[string]any{"width": 1920, "height": 1080}},
			},
			"16:9",
		},
		{
			"ImageOptions with resolution",
			workflow.Graph{
				"1": {ClassType: "ImageOptions", Inputs: map[string]any{"size": "1920x1080", "resolution": "1080P"}},
			},
			"16:9",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ExtractImageSizeSpec(tc.graph)
			if spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
		})
	}
}

func TestExtractVideoSizeSpec_AdditionalPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		graph   workflow.Graph
		wantAR  string
		wantRes string
	}{
		{
			"global width/height",
			workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"width": 1280, "height": 720}},
			},
			"16:9", "720P",
		},
		{
			"global aspect_ratio only",
			workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"aspect_ratio": "9:16"}},
			},
			"9:16", "",
		},
		{
			"global resolution only",
			workflow.Graph{
				"1": {ClassType: "Other", Inputs: map[string]any{"resolution": "1080P"}},
			},
			"", "1080P",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := ExtractVideoSizeSpec(tc.graph)
			if tc.wantAR != "" && spec.AspectRatio != tc.wantAR {
				t.Errorf("AspectRatio = %q, want %q", spec.AspectRatio, tc.wantAR)
			}
			if tc.wantRes != "" && spec.Resolution != tc.wantRes {
				t.Errorf("Resolution = %q, want %q", spec.Resolution, tc.wantRes)
			}
		})
	}
}

func TestPixelArea_ResolutionOnly(t *testing.T) {
	t.Parallel()
	spec := SizeSpec{Resolution: "720P"}
	area := spec.pixelArea()
	if area == 0 {
		t.Error("expected non-zero pixel area for resolution-only spec")
	}
}

func TestSnapTo_AspectRatioOnlySupported(t *testing.T) {
	t.Parallel()
	input := ParseDimensions(1920, 1080)
	supported := []string{"16:9", "9:16", "1:1"}
	result := input.SnapTo(supported)
	if result.AspectRatio != "16:9" {
		t.Errorf("expected 16:9, got %q", result.AspectRatio)
	}
}

func TestIsZero(t *testing.T) {
	t.Parallel()
	if !(SizeSpec{}).IsZero() {
		t.Error("empty SizeSpec should be zero")
	}
	if (SizeSpec{AspectRatio: "1:1"}).IsZero() {
		t.Error("SizeSpec with AspectRatio should not be zero")
	}
	if (SizeSpec{Resolution: "720P"}).IsZero() {
		t.Error("SizeSpec with Resolution should not be zero")
	}
}

func TestNormalizeImageSize_NoSupported(t *testing.T) {
	t.Parallel()
	// nil supported, zero dims → fallback "1024x1024"
	got := NormalizeImageSize(nil, 0, 0)
	if got != "1024x1024" {
		t.Errorf("got %q, want 1024x1024", got)
	}
}

func TestNormalizeVideoSize_SupportedNoPixels(t *testing.T) {
	t.Parallel()
	// supported entries are ratios only, no pixel dimensions extractable
	w, h := NormalizeVideoSize([]string{"16:9", "1:1"}, 0, 0)
	if w != 0 || h != 0 {
		t.Errorf("got (%d,%d), want (0,0)", w, h)
	}
}

func TestExtractImageSizeSpec_ImageOptionsNoSizeHasResolution(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "ImageOptions", Inputs: map[string]any{"aspect_ratio": "16:9", "resolution": "2K"}},
	}
	spec := ExtractImageSizeSpec(g)
	if spec.AspectRatio != "16:9" {
		t.Errorf("AspectRatio = %q, want 16:9", spec.AspectRatio)
	}
	if spec.Resolution != "2K" {
		t.Errorf("Resolution = %q, want 2K", spec.Resolution)
	}
}

func TestExtractVideoSizeSpec_GlobalAspectRatioWithResolution(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Other", Inputs: map[string]any{"aspect_ratio": "4:3", "resolution": "480P"}},
	}
	spec := ExtractVideoSizeSpec(g)
	if spec.AspectRatio != "4:3" {
		t.Errorf("AspectRatio = %q, want 4:3", spec.AspectRatio)
	}
	if spec.Resolution != "480P" {
		t.Errorf("Resolution = %q, want 480P", spec.Resolution)
	}
}

func TestAspectAngle_EmptySpec(t *testing.T) {
	t.Parallel()
	spec := SizeSpec{}
	angle := spec.aspectAngle()
	// Default is π/4 (square)
	if angle < 0.78 || angle > 0.79 {
		t.Errorf("empty spec angle = %f, want ~π/4", angle)
	}
}

func TestSnapTo_AllInvalidSupported(t *testing.T) {
	t.Parallel()
	input := ParseDimensions(1024, 1024)
	result := input.SnapTo([]string{"invalid", "garbage"})
	// Should return original since no valid candidates
	if result.Dimensions == nil || result.Dimensions.Width != 1024 {
		t.Errorf("expected original returned when all supported invalid")
	}
}

func TestSimplifyRatio_LargeNumbers(t *testing.T) {
	t.Parallel()
	// 2048x1152 = GCD gives 128:72 = too large, should approximate to 16:9
	got := simplifyRatio(2048, 1152)
	if got != "16:9" {
		t.Errorf("simplifyRatio(2048, 1152) = %q, want 16:9", got)
	}
}
