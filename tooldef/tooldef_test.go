package tooldef

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAllTools(t *testing.T) {
	t.Parallel()
	tools := AllTools()
	if len(tools) != 9 {
		t.Fatalf("expected 9 tools, got %d", len(tools))
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

func TestGenerateVideo_ReferenceImagesArray(t *testing.T) {
	t.Parallel()
	tool := GenerateVideo()
	prop, ok := tool.Parameters.Properties["reference_images"]
	if !ok {
		t.Fatal("missing reference_images property")
	}
	if prop.Type != "array" {
		t.Fatalf("reference_images type = %q, want array", prop.Type)
	}
	if prop.Items == nil || prop.Items.Type != "string" {
		t.Fatalf("reference_images items = %#v, want string items", prop.Items)
	}
}

func TestTextToSpeech_VoiceEnum(t *testing.T) {
	t.Parallel()
	tool := TextToSpeech()
	voiceProp, ok := tool.Parameters.Properties["voice"]
	if !ok {
		t.Fatal("missing voice property")
	}
	if len(voiceProp.Enum) == 0 {
		t.Fatal("voice should have enum values")
	}
	// Verify known voices are present.
	voices := map[string]bool{}
	for _, v := range voiceProp.Enum {
		voices[v] = true
	}
	for _, want := range []string{"Cherry", "Serena", "Ethan", "Chelsie"} {
		if !voices[want] {
			t.Fatalf("voice enum missing %q", want)
		}
	}
}

func TestValidateParams_RequiredMissing(t *testing.T) {
	t.Parallel()
	tool := TextToSpeech()
	err := ValidateParams(tool, map[string]interface{}{
		"voice": "Cherry",
	})
	if err == nil {
		t.Fatal("expected error for missing required 'text'")
	}
	if !strings.Contains(err.Error(), "\"text\"") {
		t.Fatalf("error should mention 'text', got: %s", err)
	}
}

func TestValidateParams_InvalidEnum(t *testing.T) {
	t.Parallel()
	tool := TextToSpeech()
	err := ValidateParams(tool, map[string]interface{}{
		"text":  "hello",
		"voice": "alloy",
	})
	if err == nil {
		t.Fatal("expected error for invalid voice enum")
	}
	if !strings.Contains(err.Error(), "alloy") || !strings.Contains(err.Error(), "Cherry") {
		t.Fatalf("error should mention invalid value and valid options, got: %s", err)
	}
}

func TestValidateParams_ValidParams(t *testing.T) {
	t.Parallel()
	tool := TextToSpeech()
	err := ValidateParams(tool, map[string]interface{}{
		"text":  "hello world",
		"voice": "Cherry",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateParams_OptionalEnumSkipped(t *testing.T) {
	t.Parallel()
	tool := GenerateImage()
	// size is optional with enum; not providing it should be fine
	err := ValidateParams(tool, map[string]interface{}{
		"prompt": "a cat",
	})
	if err != nil {
		t.Fatalf("expected no error for missing optional enum param, got: %v", err)
	}
}

func TestToolsFor_Image(t *testing.T) {
	t.Parallel()
	tools := ToolsFor("image")
	names := map[string]bool{}
	for _, t := range tools {
		names[t.Name] = true
	}
	if !names["generate_image"] || !names["edit_image"] {
		t.Fatalf("expected generate_image and edit_image, got %v", names)
	}
	if names["generate_video"] {
		t.Fatal("video tool should not be in image filter")
	}
}

func TestToolsFor_Video(t *testing.T) {
	t.Parallel()
	tools := ToolsFor("video")
	names := map[string]bool{}
	for _, t := range tools {
		names[t.Name] = true
	}
	if !names["generate_video"] || !names["edit_video"] {
		t.Fatalf("expected generate_video and edit_video, got %v", names)
	}
}

func TestToolsFor_Multiple(t *testing.T) {
	t.Parallel()
	tools := ToolsFor("audio", "music")
	names := map[string]bool{}
	for _, t := range tools {
		names[t.Name] = true
	}
	if !names["text_to_speech"] || !names["transcribe_audio"] || !names["generate_music"] {
		t.Fatalf("expected audio+music tools, got %v", names)
	}
}

func TestToolsFor_Empty(t *testing.T) {
	t.Parallel()
	tools := ToolsFor()
	if len(tools) != 0 {
		t.Fatalf("expected empty, got %d", len(tools))
	}
}

func TestToolsFor_NoMatch(t *testing.T) {
	t.Parallel()
	tools := ToolsFor("nonexistent")
	if len(tools) != 0 {
		t.Fatalf("expected empty, got %d", len(tools))
	}
}

func TestAllTools_CategorySet(t *testing.T) {
	t.Parallel()
	for _, tool := range AllTools() {
		if tool.Category == "" {
			t.Fatalf("tool %q has no category", tool.Name)
		}
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
