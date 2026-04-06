package tooldef

import (
	"encoding/json"
	"testing"
)

func TestAllTools(t *testing.T) {
	t.Parallel()
	tools := AllTools()
	if len(tools) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		if tool.Name == "" {
			t.Fatal("tool has empty name")
		}
		if tool.Description == "" {
			t.Fatalf("tool %q has empty description", tool.Name)
		}
		if tool.Parameters.Type != "object" {
			t.Fatalf("tool %q parameters type = %q, want object", tool.Name, tool.Parameters.Type)
		}
		if len(tool.Parameters.Required) == 0 {
			t.Fatalf("tool %q has no required fields", tool.Name)
		}
		if names[tool.Name] {
			t.Fatalf("duplicate tool name: %q", tool.Name)
		}
		names[tool.Name] = true
	}
}

func TestToolDefJSON(t *testing.T) {
	t.Parallel()
	for _, tool := range AllTools() {
		data, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("tool %q: marshal error: %v", tool.Name, err)
		}

		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("tool %q: unmarshal error: %v", tool.Name, err)
		}

		if decoded["name"] != tool.Name {
			t.Fatalf("tool %q: name mismatch after roundtrip", tool.Name)
		}
		params, ok := decoded["parameters"].(map[string]any)
		if !ok {
			t.Fatalf("tool %q: parameters not an object", tool.Name)
		}
		if params["type"] != "object" {
			t.Fatalf("tool %q: parameters.type = %v", tool.Name, params["type"])
		}
		props, ok := params["properties"].(map[string]any)
		if !ok || len(props) == 0 {
			t.Fatalf("tool %q: no properties", tool.Name)
		}
	}
}

func TestGenerateImage_PromptRequired(t *testing.T) {
	t.Parallel()
	tool := GenerateImage()
	found := false
	for _, r := range tool.Parameters.Required {
		if r == "prompt" {
			found = true
		}
	}
	if !found {
		t.Fatal("prompt should be required")
	}
}

func TestGenerateImage_SizeEnum(t *testing.T) {
	t.Parallel()
	tool := GenerateImage()
	sizeProp, ok := tool.Parameters.Properties["size"]
	if !ok {
		t.Fatal("missing size property")
	}
	if len(sizeProp.Enum) == 0 {
		t.Fatal("size should have enum values")
	}
}

func TestDesignVoice_RequiredFields(t *testing.T) {
	t.Parallel()
	tool := DesignVoice()
	required := map[string]bool{}
	for _, r := range tool.Parameters.Required {
		required[r] = true
	}
	for _, want := range []string{"voice_prompt", "preview_text", "target_model"} {
		if !required[want] {
			t.Fatalf("%q should be required", want)
		}
	}
}
