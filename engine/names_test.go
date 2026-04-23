package engine

import "testing"

func TestLookupDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key    string
		wantEN string
		wantZH string
	}{
		{"alibabacloud", "Alibaba Cloud DashScope", "阿里云百炼"},
		{"kling", "Kling AI", "可灵 AI"},
		{"jimeng", "Jimeng", "即梦"},
		{"hailuo", "Hailuo Video", "海螺视频"},
		{"volcvoice", "Volcengine Speech", "火山引擎语音"},
		{"embed/alibabacloud", "DashScope Embeddings", "百炼向量嵌入"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			dn := LookupDisplayName(tt.key)
			if dn["en"] != tt.wantEN {
				t.Errorf("EN = %q, want %q", dn["en"], tt.wantEN)
			}
			if dn["zh"] != tt.wantZH {
				t.Errorf("ZH = %q, want %q", dn["zh"], tt.wantZH)
			}
		})
	}
}

func TestLookupDisplayName_Unknown(t *testing.T) {
	t.Parallel()
	dn := LookupDisplayName("nonexistent")
	if dn["en"] != "nonexistent" || dn["zh"] != "nonexistent" {
		t.Errorf("expected fallback, got EN=%q ZH=%q", dn["en"], dn["zh"])
	}
}

func TestDisplayName_String(t *testing.T) {
	t.Parallel()
	dn := DisplayName{"en": "Kling AI", "zh": "可灵 AI"}
	if dn.String() != "Kling AI" {
		t.Errorf("String() = %q", dn.String())
	}
}

func TestEngineDisplayNames_AllHaveBothLanguages(t *testing.T) {
	t.Parallel()
	for key, dn := range EngineDisplayNames {
		if dn["en"] == "" {
			t.Errorf("engine %q has empty EN name", key)
		}
		if dn["zh"] == "" {
			t.Errorf("engine %q has empty ZH name", key)
		}
	}
}

func TestLookupEngineMetadata(t *testing.T) {
	t.Parallel()

	meta := LookupEngineMetadata("kling")
	if meta.DisplayName["en"] != "Kling AI" {
		t.Errorf("DisplayName[en] = %q, want %q", meta.DisplayName["en"], "Kling AI")
	}
	if meta.Intro["en"] == "" {
		t.Error("Intro[en] is empty")
	}
	if meta.Intro["zh"] == "" {
		t.Error("Intro[zh] is empty")
	}
	if meta.DocURL == "" {
		t.Error("DocURL is empty")
	}
}

func TestLookupEngineMetadata_Unknown(t *testing.T) {
	t.Parallel()
	meta := LookupEngineMetadata("nonexistent")
	if meta.DisplayName["en"] != "nonexistent" {
		t.Errorf("expected fallback DisplayName, got %q", meta.DisplayName["en"])
	}
	if meta.Intro != nil {
		t.Errorf("expected nil Intro for unknown, got %v", meta.Intro)
	}
}

func TestEngineMetadataMap_AllHaveIntro(t *testing.T) {
	t.Parallel()
	for key, meta := range EngineMetadataMap {
		if meta.DisplayName["en"] == "" {
			t.Errorf("engine %q has empty DisplayName[en]", key)
		}
		if meta.Intro["en"] == "" {
			t.Errorf("engine %q has empty Intro[en]", key)
		}
		if meta.Intro["zh"] == "" {
			t.Errorf("engine %q has empty Intro[zh]", key)
		}
	}
}

func TestEngineDisplayNamesBackwardCompat(t *testing.T) {
	t.Parallel()
	// EngineDisplayNames should contain the same keys as EngineMetadataMap.
	if len(EngineDisplayNames) != len(EngineMetadataMap) {
		t.Errorf("EngineDisplayNames has %d entries, EngineMetadataMap has %d", len(EngineDisplayNames), len(EngineMetadataMap))
	}
	for key, dn := range EngineDisplayNames {
		meta, ok := EngineMetadataMap[key]
		if !ok {
			t.Errorf("key %q in EngineDisplayNames but not in EngineMetadataMap", key)
			continue
		}
		if dn["en"] != meta.DisplayName["en"] {
			t.Errorf("key %q: EngineDisplayNames[en]=%q != EngineMetadataMap[en]=%q", key, dn["en"], meta.DisplayName["en"])
		}
	}
}
