package resolve

import (
	"encoding/json"
	"testing"

	"github.com/godeps/aigo/workflow"
)

// --- ResolveNodeString ---

func TestResolveNodeString_DirectText(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{"text": "hello world"}},
	}
	got, ok, err := ResolveNodeString(g, "1", map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestResolveNodeString_FallbackKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"prompt key", "prompt", "a prompt"},
		{"string key", "string", "a string"},
		{"value key", "value", "a value"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := workflow.Graph{
				"1": {ClassType: "T", Inputs: map[string]any{tc.key: tc.value}},
			}
			got, ok, err := ResolveNodeString(g, "1", map[string]bool{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Fatalf("expected ok=true for key %q", tc.key)
			}
			if got != tc.value {
				t.Errorf("got %q, want %q", got, tc.value)
			}
		})
	}
}

func TestResolveNodeString_LinkFollowing(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{"text": []any{"2", 0}}},
		"2": {ClassType: "T", Inputs: map[string]any{"text": "linked text"}},
	}
	got, ok, err := ResolveNodeString(g, "1", map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "linked text" {
		t.Errorf("got %q, want %q", got, "linked text")
	}
}

func TestResolveNodeString_CycleDetection(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{"text": []any{"2", 0}}},
		"2": {ClassType: "T", Inputs: map[string]any{"text": []any{"1", 0}}},
	}
	// Starting at node "1" with "1" already in seen should trigger cycle error.
	_, _, err := ResolveNodeString(g, "1", map[string]bool{"1": true})
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestResolveNodeString_NotFound(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{"text": "hi"}},
	}
	_, _, err := ResolveNodeString(g, "99", map[string]bool{})
	if err == nil {
		t.Fatal("expected error for nonexistent node, got nil")
	}
}

// --- StringOption ---

func TestStringOption(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"2": {ClassType: "T", Inputs: map[string]any{"prompt": "from node 2"}},
		"1": {ClassType: "T", Inputs: map[string]any{"prompt": "from node 1"}},
	}
	// SortedNodeIDs returns ["1","2"], so node "1" should win.
	got, ok := StringOption(g, "prompt")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "from node 1" {
		t.Errorf("got %q, want %q", got, "from node 1")
	}
}

// --- IntOption ---

func TestIntOption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input any
		want  int
	}{
		{"int value", 42, 42},
		{"float64 value", float64(7), 7},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := workflow.Graph{
				"1": {ClassType: "T", Inputs: map[string]any{"width": tc.input}},
			}
			got, ok := IntOption(g, "width")
			if !ok {
				t.Fatalf("expected ok=true for input %v", tc.input)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// --- BoolOption ---

func TestBoolOption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input any
		want  bool
	}{
		{"raw bool true", true, true},
		{"raw bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := workflow.Graph{
				"1": {ClassType: "T", Inputs: map[string]any{"flag": tc.input}},
			}
			got, ok := BoolOption(g, "flag")
			if !ok {
				t.Fatalf("expected ok=true for input %v", tc.input)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Float64Option ---

func TestFloat64Option(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input any
		want  float64
	}{
		{"float64 value", float64(3.14), 3},  // IntInput matches first, truncates to int then back to float64
		{"int value", 5, float64(5)},
		{"string value", "3.14", 3.14},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := workflow.Graph{
				"1": {ClassType: "T", Inputs: map[string]any{"cfg": tc.input}},
			}
			got, ok := Float64Option(g, "cfg")
			if !ok {
				t.Fatalf("expected ok=true for input %v", tc.input)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// --- NormalizeOpenAIImageSize ---

func TestNormalizeOpenAIImageSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		width, height int
		want          string
	}{
		{"exact 1024x1024", 1024, 1024, "1024x1024"},
		{"exact 1024x1536", 1024, 1536, "1024x1536"},
		{"exact 1536x1024", 1536, 1024, "1536x1024"},
		{"landscape wider", 2000, 1000, "1536x1024"},
		{"portrait taller", 1000, 2000, "1024x1536"},
		{"square default", 512, 512, "1024x1024"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := NormalizeOpenAIImageSize(tc.width, tc.height)
			if got != tc.want {
				t.Errorf("NormalizeOpenAIImageSize(%d, %d) = %q, want %q", tc.width, tc.height, got, tc.want)
			}
		})
	}
}

// --- ExtractPrompt ---

func TestExtractPrompt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		graph   workflow.Graph
		want    string
		wantErr bool
	}{
		{
			name: "CLIPTextEncode with direct text",
			graph: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "a beautiful sunset"}},
			},
			want: "a beautiful sunset",
		},
		{
			name: "CLIPTextEncode with link reference",
			graph: workflow.Graph{
				"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": []any{"2", 0}}},
				"2": {ClassType: "StringNode", Inputs: map[string]any{"text": "linked prompt"}},
			},
			want: "linked prompt",
		},
		{
			name: "fallback to prompt option key",
			graph: workflow.Graph{
				"1": {ClassType: "SomeOtherNode", Inputs: map[string]any{"prompt": "my prompt"}},
			},
			want: "my prompt",
		},
		{
			name: "fallback to text option key",
			graph: workflow.Graph{
				"1": {ClassType: "SomeOtherNode", Inputs: map[string]any{"text": "my text"}},
			},
			want: "my text",
		},
		{
			name: "fallback to value option key",
			graph: workflow.Graph{
				"1": {ClassType: "SomeOtherNode", Inputs: map[string]any{"value": "my value"}},
			},
			want: "my value",
		},
		{
			name:  "empty graph",
			graph: workflow.Graph{},
			want:  "",
		},
		{
			name: "graph with only non-prompt nodes",
			graph: workflow.Graph{
				"1": {ClassType: "KSampler", Inputs: map[string]any{"seed": 42}},
				"2": {ClassType: "CheckpointLoader", Inputs: map[string]any{"ckpt_name": "model.safetensors"}},
			},
			want: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractPrompt(tc.graph)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- MergeJSONOption ---

func TestMergeJSONOption(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON merges into dst", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"1": {ClassType: "T", Inputs: map[string]any{"extra": `{"model":"gpt-4","temperature":0.7}`}},
		}
		dst := map[string]any{"existing": "keep"}
		MergeJSONOption(g, dst, "extra")

		if dst["existing"] != "keep" {
			t.Error("existing key was overwritten")
		}
		if dst["model"] != "gpt-4" {
			t.Errorf("model = %v, want %q", dst["model"], "gpt-4")
		}
		if dst["temperature"] != 0.7 {
			t.Errorf("temperature = %v, want 0.7", dst["temperature"])
		}
	})

	t.Run("invalid JSON is silently skipped", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"1": {ClassType: "T", Inputs: map[string]any{"extra": "not json at all"}},
		}
		dst := map[string]any{"existing": "keep"}
		MergeJSONOption(g, dst, "extra")

		if len(dst) != 1 {
			t.Errorf("dst length = %d, want 1", len(dst))
		}
		if dst["existing"] != "keep" {
			t.Error("existing key was modified")
		}
	})

	t.Run("multiple keys merge all matches", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"1": {ClassType: "T", Inputs: map[string]any{
				"opts1": `{"a":"1"}`,
				"opts2": `{"b":"2"}`,
			}},
		}
		dst := map[string]any{}
		MergeJSONOption(g, dst, "opts1", "opts2")

		if dst["a"] != "1" {
			t.Errorf("a = %v, want %q", dst["a"], "1")
		}
		if dst["b"] != "2" {
			t.Errorf("b = %v, want %q", dst["b"], "2")
		}
	})

	t.Run("missing key is skipped", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"1": {ClassType: "T", Inputs: map[string]any{"other": "val"}},
		}
		dst := map[string]any{}
		MergeJSONOption(g, dst, "nonexistent")

		if len(dst) != 0 {
			t.Errorf("dst length = %d, want 0", len(dst))
		}
	})

	t.Run("empty graph leaves dst unchanged", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		dst := map[string]any{"k": "v"}
		MergeJSONOption(g, dst, "anything")

		if len(dst) != 1 || dst["k"] != "v" {
			t.Errorf("dst was modified: %v", dst)
		}
	})
}

// --- ResolveValueString edge cases ---

func TestResolveValueString(t *testing.T) {
	t.Parallel()

	t.Run("string value returns directly", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		got, ok, err := ResolveValueString(g, "hello", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "hello" {
			t.Errorf("got %q, want %q", got, "hello")
		}
	})

	t.Run("non-string non-slice value returns false", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		got, ok, err := ResolveValueString(g, 42, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected ok=false for int value")
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("slice value resolves via link", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"target": {ClassType: "T", Inputs: map[string]any{"text": "resolved"}},
		}
		got, ok, err := ResolveValueString(g, []any{"target", 0}, map[string]bool{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "resolved" {
			t.Errorf("got %q, want %q", got, "resolved")
		}
	})
}

// --- ResolveLinkString edge cases ---

func TestResolveLinkString(t *testing.T) {
	t.Parallel()

	t.Run("empty ref returns false", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		got, ok, err := ResolveLinkString(g, []any{}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected ok=false for empty ref")
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("nil ref returns false", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		got, ok, err := ResolveLinkString(g, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected ok=false for nil ref")
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("non-string first element returns false", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{}
		got, ok, err := ResolveLinkString(g, []any{123, 0}, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected ok=false for non-string first element")
		}
		if got != "" {
			t.Errorf("got %q, want empty string", got)
		}
	})

	t.Run("valid ref resolves to node output", func(t *testing.T) {
		t.Parallel()

		g := workflow.Graph{
			"n1": {ClassType: "T", Inputs: map[string]any{"text": "found it"}},
		}
		got, ok, err := ResolveLinkString(g, []any{"n1", 0}, map[string]bool{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "found it" {
			t.Errorf("got %q, want %q", got, "found it")
		}
	})
}

// --- StringOption edge case: multiple keys, first-found priority ---

func TestStringOption_MultipleKeys(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{
			"prompt": "the prompt",
			"text":   "the text",
		}},
	}

	// "prompt" is listed first, so it should be returned even though "text" also exists.
	got, ok := StringOption(g, "prompt", "text")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "the prompt" {
		t.Errorf("got %q, want %q", got, "the prompt")
	}

	// When first key is absent, second key should be returned.
	got2, ok2 := StringOption(g, "nonexistent", "text")
	if !ok2 {
		t.Fatal("expected ok=true for fallback key")
	}
	if got2 != "the text" {
		t.Errorf("got %q, want %q", got2, "the text")
	}
}

// --- BoolOption edge case: non-parseable string ---

func TestBoolOption_NonParseableString(t *testing.T) {
	t.Parallel()

	g := workflow.Graph{
		"1": {ClassType: "T", Inputs: map[string]any{"flag": "maybe"}},
	}
	got, ok := BoolOption(g, "flag")
	if ok {
		t.Fatal("expected ok=false for non-parseable bool string")
	}
	if got != false {
		t.Errorf("got %v, want false", got)
	}
}

// --- Float64Option edge case: json.Number ---

func TestFloat64Option_JSONNumber(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		num  json.Number
		want float64
	}{
		{"integer json.Number", json.Number("42"), 42},
		{"fractional json.Number", json.Number("3.14"), 3}, // IntInput matches first, truncates
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := workflow.Graph{
				"1": {ClassType: "T", Inputs: map[string]any{"cfg": tc.num}},
			}
			got, ok := Float64Option(g, "cfg")
			if !ok {
				t.Fatalf("expected ok=true for json.Number(%q)", tc.num)
			}
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
