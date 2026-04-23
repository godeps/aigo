package engine

import (
	"encoding/json"
	"testing"
)

func TestRegisterAndLookupModelInfo(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{
			Name:        "test-model-alpha",
			DisplayName: DisplayName{"en": "Alpha", "zh": "阿尔法"},
			Description: DisplayName{"en": "Test model", "zh": "测试模型"},
			Capability:  "image",
		},
	})

	info, ok := LookupModelInfo("test-model-alpha")
	if !ok {
		t.Fatal("LookupModelInfo() returned false for registered model")
	}
	if info.DisplayName["en"] != "Alpha" {
		t.Errorf("DisplayName[en] = %q, want %q", info.DisplayName["en"], "Alpha")
	}
	if info.DisplayName["zh"] != "阿尔法" {
		t.Errorf("DisplayName[zh] = %q, want %q", info.DisplayName["zh"], "阿尔法")
	}
	if info.Capability != "image" {
		t.Errorf("Capability = %q, want %q", info.Capability, "image")
	}
}

func TestLookupModelInfoNotFound(t *testing.T) {
	t.Parallel()

	_, ok := LookupModelInfo("nonexistent-model-xyz")
	if ok {
		t.Fatal("LookupModelInfo() returned true for unknown model")
	}
}

func TestAllModelInfosSorted(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-zzz-model", Capability: "video"},
		{Name: "test-aaa-model", Capability: "image"},
	})

	all := AllModelInfos()
	if len(all) < 2 {
		t.Fatal("AllModelInfos() returned fewer than 2 entries")
	}

	// Verify sorted order.
	for i := 1; i < len(all); i++ {
		if all[i].Name < all[i-1].Name {
			t.Errorf("AllModelInfos() not sorted: %q before %q", all[i-1].Name, all[i].Name)
			break
		}
	}
}

func TestModelInfosByCapability(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-cap-img-1", Provider: "test-pkg", Capability: "image"},
		{Name: "test-cap-vid-1", Provider: "test-pkg", Capability: "video"},
		{Name: "test-cap-img-2", Provider: "test-pkg", Capability: "image"},
	})

	images := ModelInfosByCapability("image")
	found := 0
	for _, info := range images {
		if info.Name == "test-cap-img-1" || info.Name == "test-cap-img-2" {
			found++
		}
		if info.Capability != "image" {
			t.Errorf("ModelInfosByCapability(image) returned %q with cap %q", info.Name, info.Capability)
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 image models, found %d", found)
	}
}

func TestModelInfosByProvider(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-prov-a1", Provider: "test-provider-a", Capability: "image"},
		{Name: "test-prov-a2", Provider: "test-provider-a", Capability: "video"},
		{Name: "test-prov-b1", Provider: "test-provider-b", Capability: "image"},
	})

	provA := ModelInfosByProvider("test-provider-a")
	found := 0
	for _, info := range provA {
		if info.Name == "test-prov-a1" || info.Name == "test-prov-a2" {
			found++
		}
		if info.Provider != "test-provider-a" {
			t.Errorf("ModelInfosByProvider returned %q with provider %q", info.Name, info.Provider)
		}
	}
	if found < 2 {
		t.Errorf("expected at least 2 models for test-provider-a, found %d", found)
	}

	empty := ModelInfosByProvider("nonexistent-provider")
	if len(empty) != 0 {
		t.Errorf("expected empty for nonexistent provider, got %d", len(empty))
	}
}

func TestSearchModelInfos(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-search-flux-pro", DisplayName: DisplayName{"en": "FLUX Pro", "zh": "FLUX 专业版"}, Capability: "image"},
		{Name: "test-search-other", DisplayName: DisplayName{"en": "Other Model", "zh": "其他模型"}, Capability: "video"},
	})

	// Search by name.
	results := SearchModelInfos("test-search-flux")
	found := false
	for _, r := range results {
		if r.Name == "test-search-flux-pro" {
			found = true
		}
	}
	if !found {
		t.Error("SearchModelInfos(flux) did not find test-search-flux-pro")
	}

	// Search by display name (case-insensitive).
	results = SearchModelInfos("专业版")
	found = false
	for _, r := range results {
		if r.Name == "test-search-flux-pro" {
			found = true
		}
	}
	if !found {
		t.Error("SearchModelInfos(专业版) did not find test-search-flux-pro by zh DisplayName")
	}

	// No results.
	results = SearchModelInfos("nonexistent-search-xyz-12345")
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestExportModelCatalog(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-export-model", Capability: "image"},
	})

	data, err := ExportModelCatalog()
	if err != nil {
		t.Fatalf("ExportModelCatalog() error: %v", err)
	}

	var models []ModelInfo
	if err := json.Unmarshal(data, &models); err != nil {
		t.Fatalf("unmarshal catalog: %v", err)
	}
	if len(models) == 0 {
		t.Error("exported catalog is empty")
	}

	// Verify sorted.
	for i := 1; i < len(models); i++ {
		if models[i].Name < models[i-1].Name {
			t.Errorf("catalog not sorted: %q before %q", models[i-1].Name, models[i].Name)
			break
		}
	}
}

func TestRegisterModelInfosOverwrite(t *testing.T) {
	t.Parallel()

	RegisterModelInfos([]ModelInfo{
		{Name: "test-overwrite-model", DisplayName: DisplayName{"en": "Old"}, Capability: "image"},
	})
	RegisterModelInfos([]ModelInfo{
		{Name: "test-overwrite-model", DisplayName: DisplayName{"en": "New"}, Capability: "video"},
	})

	info, ok := LookupModelInfo("test-overwrite-model")
	if !ok {
		t.Fatal("LookupModelInfo() returned false after overwrite")
	}
	if info.DisplayName["en"] != "New" {
		t.Errorf("DisplayName[en] = %q, want %q after overwrite", info.DisplayName["en"], "New")
	}
	if info.Capability != "video" {
		t.Errorf("Capability = %q, want %q after overwrite", info.Capability, "video")
	}
}
