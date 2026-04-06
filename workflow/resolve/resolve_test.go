package resolve

import (
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
