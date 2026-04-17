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
			if dn.EN != tt.wantEN {
				t.Errorf("EN = %q, want %q", dn.EN, tt.wantEN)
			}
			if dn.ZH != tt.wantZH {
				t.Errorf("ZH = %q, want %q", dn.ZH, tt.wantZH)
			}
		})
	}
}

func TestLookupDisplayName_Unknown(t *testing.T) {
	t.Parallel()
	dn := LookupDisplayName("nonexistent")
	if dn.EN != "nonexistent" || dn.ZH != "nonexistent" {
		t.Errorf("expected fallback, got EN=%q ZH=%q", dn.EN, dn.ZH)
	}
}

func TestDisplayName_String(t *testing.T) {
	t.Parallel()
	dn := DisplayName{EN: "Kling AI", ZH: "可灵 AI"}
	if dn.String() != "Kling AI" {
		t.Errorf("String() = %q", dn.String())
	}
}

func TestEngineDisplayNames_AllHaveBothLanguages(t *testing.T) {
	t.Parallel()
	for key, dn := range EngineDisplayNames {
		if dn.EN == "" {
			t.Errorf("engine %q has empty EN name", key)
		}
		if dn.ZH == "" {
			t.Errorf("engine %q has empty ZH name", key)
		}
	}
}
