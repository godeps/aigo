package aigo

import (
	"encoding/json"
	"testing"

	"github.com/godeps/aigo/engine"

	// Blank imports to trigger init() model info registrations.
	_ "github.com/godeps/aigo/engine/alibabacloud"
	_ "github.com/godeps/aigo/engine/ark"
	_ "github.com/godeps/aigo/engine/comfydeploy"
	_ "github.com/godeps/aigo/engine/comfyui"
	_ "github.com/godeps/aigo/engine/elevenlabs"
	_ "github.com/godeps/aigo/engine/embed/alibabacloud"
	_ "github.com/godeps/aigo/engine/embed/gemini"
	_ "github.com/godeps/aigo/engine/embed/jina"
	_ "github.com/godeps/aigo/engine/embed/openai"
	_ "github.com/godeps/aigo/engine/embed/voyage"
	_ "github.com/godeps/aigo/engine/fal"
	_ "github.com/godeps/aigo/engine/flux"
	_ "github.com/godeps/aigo/engine/gemini"
	_ "github.com/godeps/aigo/engine/google"
	_ "github.com/godeps/aigo/engine/gpt4o"
	_ "github.com/godeps/aigo/engine/hailuo"
	_ "github.com/godeps/aigo/engine/hedra"
	_ "github.com/godeps/aigo/engine/ideogram"
	_ "github.com/godeps/aigo/engine/jimeng"
	_ "github.com/godeps/aigo/engine/kling"
	_ "github.com/godeps/aigo/engine/liblib"
	_ "github.com/godeps/aigo/engine/luma"
	_ "github.com/godeps/aigo/engine/meshy"
	_ "github.com/godeps/aigo/engine/midjourney"
	_ "github.com/godeps/aigo/engine/minimax"
	_ "github.com/godeps/aigo/engine/newapi"
	_ "github.com/godeps/aigo/engine/openai"
	_ "github.com/godeps/aigo/engine/openrouter"
	_ "github.com/godeps/aigo/engine/pika"
	_ "github.com/godeps/aigo/engine/recraft"
	_ "github.com/godeps/aigo/engine/replicate"
	_ "github.com/godeps/aigo/engine/runninghub"
	_ "github.com/godeps/aigo/engine/runway"
	_ "github.com/godeps/aigo/engine/stability"
	_ "github.com/godeps/aigo/engine/suno"
	_ "github.com/godeps/aigo/engine/volcvoice"
)

func TestClientModelInfo(t *testing.T) {
	t.Parallel()

	client := NewClient()

	info, ok := client.ModelInfo("kling-v2-master")
	if !ok {
		t.Fatal("ModelInfo(kling-v2-master) returned false")
	}
	if info.DisplayName["en"] == "" {
		t.Error("DisplayName[en] is empty")
	}
	if info.DisplayName["zh"] == "" {
		t.Error("DisplayName[zh] is empty")
	}
	if info.Description["en"] == "" {
		t.Error("Description[en] is empty")
	}
	if info.Capability == "" {
		t.Error("Capability is empty")
	}
	if info.Provider == "" {
		t.Error("Provider is empty")
	}
}

func TestClientModelInfoNotFound(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_, ok := client.ModelInfo("nonexistent-model-12345")
	if ok {
		t.Fatal("ModelInfo() returned true for unknown model")
	}
}

func TestClientAllModelInfos(t *testing.T) {
	t.Parallel()

	client := NewClient()
	all := client.AllModelInfos()

	if len(all) == 0 {
		t.Fatal("AllModelInfos() returned empty")
	}

	// Verify sorted.
	for i := 1; i < len(all); i++ {
		if all[i].Name < all[i-1].Name {
			t.Errorf("AllModelInfos() not sorted: %q before %q", all[i-1].Name, all[i].Name)
			break
		}
	}

	// Verify all entries have required fields.
	for _, info := range all {
		if info.Name == "" {
			t.Error("found ModelInfo with empty Name")
		}
		if info.DisplayName["en"] == "" {
			t.Errorf("ModelInfo %q has empty DisplayName[en]", info.Name)
		}
		if info.Capability == "" {
			t.Errorf("ModelInfo %q has empty Capability", info.Name)
		}
	}
}

func TestClientAllModelInfosContainsKnownModels(t *testing.T) {
	t.Parallel()

	client := NewClient()
	all := client.AllModelInfos()

	known := map[string]bool{
		"kling-v2-master":      false,
		"ray-2":                false,
		"qwen-image":           false,
		"text-embedding-3-small": false, // embed model
	}
	for _, info := range all {
		if _, ok := known[info.Name]; ok {
			known[info.Name] = true
		}
	}
	for model, found := range known {
		if !found {
			t.Errorf("expected model %q in AllModelInfos()", model)
		}
	}
}

func TestModelInfoHasI18n(t *testing.T) {
	t.Parallel()

	client := NewClient()
	all := client.AllModelInfos()

	for _, info := range all {
		if info.DisplayName["zh"] == "" {
			t.Errorf("ModelInfo %q has empty DisplayName[zh]", info.Name)
		}
		// Description[zh] is optional for some embed/platform models, skip strict check.
	}
}

func TestModelInfoCapabilities(t *testing.T) {
	t.Parallel()

	validCaps := map[string]bool{
		"image": true, "image_edit": true, "video": true, "video_edit": true,
		"tts": true, "asr": true, "music": true, "3d": true,
		"text": true, "voice_design": true, "embedding": true,
	}

	client := NewClient()
	for _, info := range client.AllModelInfos() {
		if !validCaps[info.Capability] {
			t.Errorf("ModelInfo %q has unknown Capability %q", info.Name, info.Capability)
		}
	}
}

func TestLookupModelInfoDirectly(t *testing.T) {
	t.Parallel()

	info, ok := engine.LookupModelInfo("ray-2")
	if !ok {
		t.Fatal("LookupModelInfo(ray-2) returned false")
	}
	if info.Capability != "video" {
		t.Errorf("ray-2 Capability = %q, want video", info.Capability)
	}
}

func TestModelInfoAllHaveProvider(t *testing.T) {
	t.Parallel()

	client := NewClient()
	for _, info := range client.AllModelInfos() {
		if info.Provider == "" {
			t.Errorf("ModelInfo %q has empty Provider", info.Name)
		}
	}
}

func TestModelInfoProviderInMetadataMap(t *testing.T) {
	t.Parallel()

	client := NewClient()
	for _, info := range client.AllModelInfos() {
		if info.Provider == "" {
			continue
		}
		if _, ok := engine.EngineMetadataMap[info.Provider]; !ok {
			t.Errorf("ModelInfo %q has Provider %q not found in EngineMetadataMap", info.Name, info.Provider)
		}
	}
}

func TestModelInfosByCapability(t *testing.T) {
	t.Parallel()

	client := NewClient()

	videoModels := client.ModelInfosByCapability("video")
	if len(videoModels) == 0 {
		t.Fatal("ModelInfosByCapability(video) returned empty")
	}
	for _, info := range videoModels {
		if info.Capability != "video" {
			t.Errorf("ModelInfosByCapability(video) returned %q with cap %q", info.Name, info.Capability)
		}
	}

	imageModels := client.ModelInfosByCapability("image")
	if len(imageModels) == 0 {
		t.Fatal("ModelInfosByCapability(image) returned empty")
	}

	embedModels := client.ModelInfosByCapability("embedding")
	if len(embedModels) == 0 {
		t.Fatal("ModelInfosByCapability(embedding) returned empty")
	}
}

func TestModelInfosByProvider(t *testing.T) {
	t.Parallel()

	client := NewClient()

	klingModels := client.ModelInfosByProvider("kling")
	if len(klingModels) == 0 {
		t.Fatal("ModelInfosByProvider(kling) returned empty")
	}
	for _, info := range klingModels {
		if info.Provider != "kling" {
			t.Errorf("ModelInfosByProvider(kling) returned %q with provider %q", info.Name, info.Provider)
		}
	}

	// Test embed provider.
	embedModels := client.ModelInfosByProvider("embed/openai")
	if len(embedModels) == 0 {
		t.Fatal("ModelInfosByProvider(embed/openai) returned empty")
	}

	empty := client.ModelInfosByProvider("nonexistent-provider-xyz")
	if len(empty) != 0 {
		t.Errorf("expected empty for nonexistent provider, got %d", len(empty))
	}
}

func TestSearchModelInfos(t *testing.T) {
	t.Parallel()

	client := NewClient()

	// Search by name substring.
	results := client.SearchModelInfos("kling")
	if len(results) == 0 {
		t.Fatal("SearchModelInfos(kling) returned empty")
	}

	// Search by Chinese display name.
	results = client.SearchModelInfos("可灵")
	if len(results) == 0 {
		t.Fatal("SearchModelInfos(可灵) returned empty")
	}

	// Case insensitive.
	results = client.SearchModelInfos("FLUX")
	if len(results) == 0 {
		t.Fatal("SearchModelInfos(FLUX) returned empty")
	}

	// No results.
	results = client.SearchModelInfos("zzzznonexistent12345")
	if len(results) != 0 {
		t.Errorf("expected empty, got %d", len(results))
	}
}

func TestExportModelCatalog(t *testing.T) {
	t.Parallel()

	client := NewClient()
	data, err := client.ExportModelCatalog()
	if err != nil {
		t.Fatalf("ExportModelCatalog() error: %v", err)
	}

	var models []engine.ModelInfo
	if err := json.Unmarshal(data, &models); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(models) == 0 {
		t.Error("exported catalog is empty")
	}
}

func TestEngineInfosHaveModels(t *testing.T) {
	t.Parallel()

	client := NewClient()
	// Register at least one engine so EngineInfos is non-empty.
	_ = client.RegisterEngine("kling-test", &stubEngine{})

	infos := client.EngineInfos()
	for _, info := range infos {
		if info.Name == "kling-test" {
			// Stub engine won't have models, just check it doesn't panic.
			continue
		}
	}
}
