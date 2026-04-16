package aigo

import (
	"context"
	"errors"
	"testing"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/workflow"
)

func TestInferMediaType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task AgentTask
		want string
	}{
		{"default is image", AgentTask{Prompt: "a cat"}, "image"},
		{"duration means video", AgentTask{Prompt: "p", Duration: 5}, "video"},
		{"video reference means video", AgentTask{
			Prompt:     "p",
			References: []ReferenceAsset{{Type: ReferenceTypeVideo, URL: "https://x/v.mp4"}},
		}, "video"},
		{"structured video duration", AgentTask{
			Prompt:     "p",
			Structured: &AgentTaskStructured{VideoDuration: 3},
		}, "video"},
		{"structured video size", AgentTask{
			Prompt:     "p",
			Structured: &AgentTaskStructured{VideoSize: "1280*720"},
		}, "video"},
		{"structured video aspect ratio", AgentTask{
			Prompt:     "p",
			Structured: &AgentTaskStructured{VideoAspectRatio: "16:9"},
		}, "video"},
		{"TTS means audio", AgentTask{Prompt: "hello", TTS: &TTSOptions{Voice: "Cherry"}}, "audio"},
		{"music means music", AgentTask{Prompt: "p", Music: &MusicOptions{Lyrics: "la la"}}, "music"},
		{"voice design", AgentTask{
			Prompt:      "p",
			VoiceDesign: &VoiceDesignOptions{VoicePrompt: "warm", PreviewText: "hi", TargetModel: "m"},
		}, "voice_design"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InferMediaType(tt.task)
			if got != tt.want {
				t.Fatalf("InferMediaType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuleFilter_MediaType(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "img-engine", Capability: engine.Capability{MediaTypes: []string{"image"}}},
		{Name: "vid-engine", Capability: engine.Capability{MediaTypes: []string{"video"}}},
		{Name: "both-engine", Capability: engine.Capability{MediaTypes: []string{"image", "video"}}},
		{Name: "no-cap", Capability: engine.Capability{}}, // no metadata → assumed capable
	}

	f := &RuleFilter{}

	// Image task should match img-engine, both-engine, and no-cap.
	got := f.Filter(AgentTask{Prompt: "cat"}, candidates)
	names := infoNames(got)
	assertContains(t, names, "img-engine")
	assertContains(t, names, "both-engine")
	assertContains(t, names, "no-cap")
	assertNotContains(t, names, "vid-engine")

	// Video task should match vid-engine, both-engine, and no-cap.
	got = f.Filter(AgentTask{Prompt: "p", Duration: 5}, candidates)
	names = infoNames(got)
	assertContains(t, names, "vid-engine")
	assertContains(t, names, "both-engine")
	assertContains(t, names, "no-cap")
	assertNotContains(t, names, "img-engine")
}

func TestRuleFilter_Duration(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "short", Capability: engine.Capability{MediaTypes: []string{"video"}, MaxDuration: 5}},
		{Name: "long", Capability: engine.Capability{MediaTypes: []string{"video"}, MaxDuration: 60}},
		{Name: "unlimited", Capability: engine.Capability{MediaTypes: []string{"video"}}}, // MaxDuration=0 → no limit
	}

	f := &RuleFilter{}

	// 10s task: "short" should be excluded.
	got := f.Filter(AgentTask{Prompt: "p", Duration: 10}, candidates)
	names := infoNames(got)
	assertNotContains(t, names, "short")
	assertContains(t, names, "long")
	assertContains(t, names, "unlimited")
}

func TestRuleFilter_Voice(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "tts-a", Capability: engine.Capability{MediaTypes: []string{"audio"}, Voices: []string{"Cherry", "Serena"}}},
		{Name: "tts-b", Capability: engine.Capability{MediaTypes: []string{"audio"}, Voices: []string{"Ethan"}}},
		{Name: "tts-any", Capability: engine.Capability{MediaTypes: []string{"audio"}}}, // no voice metadata
	}

	f := &RuleFilter{}
	task := AgentTask{Prompt: "hello", TTS: &TTSOptions{Voice: "Cherry"}}

	got := f.Filter(task, candidates)
	names := infoNames(got)
	assertContains(t, names, "tts-a")
	assertContains(t, names, "tts-any")
	assertNotContains(t, names, "tts-b")
}

func TestRuleFilter_Size(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "small", Capability: engine.Capability{MediaTypes: []string{"image"}, Sizes: []string{"512x512"}}},
		{Name: "big", Capability: engine.Capability{MediaTypes: []string{"image"}, Sizes: []string{"1024x1024", "1536x1024"}}},
		{Name: "any", Capability: engine.Capability{MediaTypes: []string{"image"}}}, // no size metadata
	}

	f := &RuleFilter{}
	task := AgentTask{Prompt: "p", Size: "1024x1024"}

	got := f.Filter(task, candidates)
	names := infoNames(got)
	assertContains(t, names, "big")
	assertContains(t, names, "any")
	assertNotContains(t, names, "small")
}

func TestRuleFilter_StructuredSize(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "hd", Capability: engine.Capability{MediaTypes: []string{"video"}, Sizes: []string{"1280*720", "1920*1080"}}},
		{Name: "sd", Capability: engine.Capability{MediaTypes: []string{"video"}, Sizes: []string{"640*480"}}},
	}

	f := &RuleFilter{}
	task := AgentTask{
		Prompt:     "p",
		Structured: &AgentTaskStructured{VideoDuration: 5, VideoSize: "1280*720"},
	}

	got := f.Filter(task, candidates)
	names := infoNames(got)
	assertContains(t, names, "hd")
	assertNotContains(t, names, "sd")
}

func TestRuleFilter_EmptyCandidates(t *testing.T) {
	t.Parallel()
	f := &RuleFilter{}
	got := f.Filter(AgentTask{Prompt: "p"}, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
}

func TestPrioritySelector_SelectEngine(t *testing.T) {
	t.Parallel()

	s := &PrioritySelector{Priority: []string{"preferred", "backup"}}
	sel, err := s.SelectEngine(context.Background(), AgentTask{Prompt: "p"}, []string{"backup", "other", "preferred"})
	if err != nil {
		t.Fatal(err)
	}
	if sel.Engine != "preferred" {
		t.Fatalf("got %q, want %q", sel.Engine, "preferred")
	}
}

func TestPrioritySelector_FallbackToFirst(t *testing.T) {
	t.Parallel()

	s := &PrioritySelector{Priority: []string{"not-registered"}}
	sel, err := s.SelectEngine(context.Background(), AgentTask{Prompt: "p"}, []string{"beta", "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	// Should fall back to first alphabetical.
	if sel.Engine != "alpha" {
		t.Fatalf("got %q, want %q", sel.Engine, "alpha")
	}
}

func TestPrioritySelector_NoEngines(t *testing.T) {
	t.Parallel()

	s := &PrioritySelector{}
	_, err := s.SelectEngine(context.Background(), AgentTask{Prompt: "p"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrioritySelector_SelectEngineFromCandidates(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "img-a", Capability: engine.Capability{MediaTypes: []string{"image"}}},
		{Name: "vid-a", Capability: engine.Capability{MediaTypes: []string{"video"}}},
		{Name: "img-b", Capability: engine.Capability{MediaTypes: []string{"image"}}},
	}

	s := &PrioritySelector{
		Priority: []string{"img-b", "img-a"},
		Filter:   &RuleFilter{},
	}

	// Image task → filter keeps img-a, img-b → priority picks img-b.
	sel, err := s.SelectEngineFromCandidates(context.Background(), AgentTask{Prompt: "cat"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Engine != "img-b" {
		t.Fatalf("got %q, want %q", sel.Engine, "img-b")
	}
}

func TestPrioritySelector_FilterRemovesAll(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "vid-only", Capability: engine.Capability{MediaTypes: []string{"video"}}},
	}

	s := &PrioritySelector{
		Priority: []string{"vid-only"},
		Filter:   &RuleFilter{},
	}

	// Image task → vid-only filtered out → error.
	_, err := s.SelectEngineFromCandidates(context.Background(), AgentTask{Prompt: "cat"}, candidates)
	if err == nil {
		t.Fatal("expected error when all candidates filtered")
	}
}

func TestPrioritySelector_NoPriorityMatch_FallbackToFiltered(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "img-x", Capability: engine.Capability{MediaTypes: []string{"image"}}},
	}

	s := &PrioritySelector{
		Priority: []string{"not-exist"},
		Filter:   &RuleFilter{},
	}

	sel, err := s.SelectEngineFromCandidates(context.Background(), AgentTask{Prompt: "cat"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if sel.Engine != "img-x" {
		t.Fatalf("got %q, want %q", sel.Engine, "img-x")
	}
}

func TestPrioritySelector_WithoutFilter(t *testing.T) {
	t.Parallel()

	candidates := []EngineInfo{
		{Name: "a", Capability: engine.Capability{MediaTypes: []string{"video"}}},
		{Name: "b", Capability: engine.Capability{MediaTypes: []string{"image"}}},
	}

	s := &PrioritySelector{Priority: []string{"b", "a"}} // no Filter

	sel, err := s.SelectEngineFromCandidates(context.Background(), AgentTask{Prompt: "cat"}, candidates)
	if err != nil {
		t.Fatal(err)
	}
	// Without filter, "b" matches first (no filtering applied).
	if sel.Engine != "b" {
		t.Fatalf("got %q, want %q", sel.Engine, "b")
	}
}

// stubDescriberEngine implements both Engine and Describer.
type stubDescriberEngine struct {
	result string
	cap    engine.Capability
}

func (s stubDescriberEngine) Execute(context.Context, workflow.Graph) (engine.Result, error) {
	return engine.Result{Value: s.result}, nil
}

func (s stubDescriberEngine) Capabilities() engine.Capability {
	return s.cap
}

func TestClientEngineInfos(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("plain", stubEngine{result: "ok"})
	_ = client.RegisterEngine("described", stubDescriberEngine{
		result: "ok",
		cap:    engine.Capability{MediaTypes: []string{"image"}, Models: []string{"flux"}},
	})

	infos := client.EngineInfos()
	if len(infos) != 2 {
		t.Fatalf("expected 2 infos, got %d", len(infos))
	}

	// Sorted by name: "described" < "plain"
	if infos[0].Name != "described" {
		t.Fatalf("first = %q", infos[0].Name)
	}
	if len(infos[0].Capability.MediaTypes) != 1 || infos[0].Capability.MediaTypes[0] != "image" {
		t.Fatalf("described cap = %+v", infos[0].Capability)
	}

	if infos[1].Name != "plain" {
		t.Fatalf("second = %q", infos[1].Name)
	}
	if len(infos[1].Capability.MediaTypes) != 0 {
		t.Fatalf("plain should have empty cap, got %+v", infos[1].Capability)
	}
}

// stubRichSelector tracks whether SelectEngineFromCandidates was called.
type stubRichSelector struct {
	gotCandidates []EngineInfo
	result        Selection
	err           error
}

func (s *stubRichSelector) SelectEngine(_ context.Context, _ AgentTask, _ []string) (Selection, error) {
	return Selection{}, errors.New("should not be called")
}

func (s *stubRichSelector) SelectEngineFromCandidates(_ context.Context, _ AgentTask, candidates []EngineInfo) (Selection, error) {
	s.gotCandidates = candidates
	return s.result, s.err
}

func TestExecuteTaskAuto_RichSelector(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("img", stubDescriberEngine{
		result: "image-url",
		cap:    engine.Capability{MediaTypes: []string{"image"}},
	})
	_ = client.RegisterEngine("vid", stubDescriberEngine{
		result: "video-url",
		cap:    engine.Capability{MediaTypes: []string{"video"}},
	})

	rs := &stubRichSelector{
		result: Selection{Engine: "vid", Reason: "task needs video"},
	}

	result, err := client.ExecuteTaskAuto(context.Background(), rs, AgentTask{Prompt: "animate", Duration: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Engine != "vid" || result.Output.Value != "video-url" {
		t.Fatalf("result = %+v", result)
	}

	// Verify RichSelector received candidates with capabilities.
	if len(rs.gotCandidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(rs.gotCandidates))
	}
	for _, c := range rs.gotCandidates {
		if len(c.Capability.MediaTypes) == 0 {
			t.Fatalf("candidate %q missing capabilities", c.Name)
		}
	}
}

func TestExecuteTaskAuto_FallbackToPlainSelector(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("e1", stubEngine{result: "ok"})

	plain := &stubSelector{result: Selection{Engine: "e1"}}
	result, err := client.ExecuteTaskAuto(context.Background(), plain, AgentTask{Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Engine != "e1" {
		t.Fatalf("engine = %q", result.Engine)
	}
	// Plain selector should have received engine names, not candidates.
	if len(plain.gotEngines) != 1 || plain.gotEngines[0] != "e1" {
		t.Fatalf("plain selector got engines %v", plain.gotEngines)
	}
}

func TestExecuteTaskAutoWithFallback(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("bad", stubEngine{err: errors.New("fail")})
	_ = client.RegisterEngine("good", stubEngine{result: "ok"})

	rs := &stubRichSelector{
		result: Selection{Engine: "bad", Reason: "preferred"},
	}

	result, err := client.ExecuteTaskAutoWithFallback(context.Background(), rs, AgentTask{Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	// "bad" fails, falls back to "good".
	if result.Engine != "good" {
		t.Fatalf("engine = %q, want good", result.Engine)
	}
}

// --- helpers ---

func infoNames(infos []EngineInfo) []string {
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name
	}
	return names
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			return
		}
	}
	t.Fatalf("expected %v to contain %q", haystack, needle)
}

func assertNotContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			t.Fatalf("expected %v to NOT contain %q", haystack, needle)
		}
	}
}
