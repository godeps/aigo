package alibabacloud

import (
	"testing"

	"github.com/godeps/aigo/engine"
)

// TestDefaultWaitForModel pins which DashScope model families default to
// blocking polling. The bug we're guarding against: video/3d/asr-filetrans
// models hand back a task UUID synchronously, and without polling that UUID
// gets written into the canvas as if it were a media URL.
func TestDefaultWaitForModel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model string
		want  bool
	}{
		// Async: defaults should poll.
		{ModelWanTextToVideo, true},
		{ModelWanImageToVideo, true},
		{ModelWanReferenceVideo, true},
		{ModelWanVideoEdit, true},
		{ModelKlingV3Video, true},
		{ModelKlingV3OmniVideo, true},
		{ModelTripoP1, true},
		{ModelTripoH31, true},
		{ModelQwenASRFlashFiletrans, true},
		{ModelQwenImageEditPlus, true}, // listed in editModels

		// Sync: defaults should NOT block.
		{ModelQwenImage, false},
		{"", false},
		{"some-future-sync-model", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.model, func(t *testing.T) {
			t.Parallel()
			if got := defaultWaitForModel(c.model); got != c.want {
				t.Fatalf("defaultWaitForModel(%q) = %v, want %v", c.model, got, c.want)
			}
		})
	}
}

// TestFactoryRespectsExplicitWaitForCompletion verifies that a user setting
// in EngineConfig overrides the model-based smart default in both directions.
func TestFactoryRespectsExplicitWaitForCompletion(t *testing.T) {
	t.Parallel()
	factory, ok := engine.GetFactory("alibabacloud")
	if !ok {
		t.Fatal("alibabacloud factory not registered")
	}

	tt := true
	ff := false

	cases := []struct {
		name string
		cfg  engine.EngineConfig
		want bool
	}{
		{
			name: "default for video model is true",
			cfg:  engine.EngineConfig{Model: ModelWanTextToVideo},
			want: true,
		},
		{
			name: "user can force false even on async video model",
			cfg:  engine.EngineConfig{Model: ModelWanTextToVideo, WaitForCompletion: &ff},
			want: false,
		},
		{
			name: "user can force true on sync image model",
			cfg:  engine.EngineConfig{Model: ModelQwenImage, WaitForCompletion: &tt},
			want: true,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			eng, err := factory(c.cfg)
			if err != nil {
				t.Fatalf("factory: %v", err)
			}
			impl, ok := eng.(*Engine)
			if !ok {
				t.Fatalf("factory returned unexpected type %T", eng)
			}
			if impl.rt.WaitForCompletion != c.want {
				t.Fatalf("WaitForCompletion = %v, want %v", impl.rt.WaitForCompletion, c.want)
			}
		})
	}
}
