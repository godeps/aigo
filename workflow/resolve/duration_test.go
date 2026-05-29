package resolve

import (
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestClampDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value      float64
		min, max   float64
		want       float64
	}{
		{"within range", 5, 1, 10, 5},
		{"below min", 0.5, 1, 10, 1},
		{"above max", 15, 1, 10, 10},
		{"no max", 100, 1, 0, 100},
		{"no min", 0.1, 0, 10, 0.1},
		{"exact min", 1, 1, 10, 1},
		{"exact max", 10, 1, 10, 10},
		{"no bounds", 999, 0, 0, 999},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ClampDuration(tc.value, tc.min, tc.max)
			if got != tc.want {
				t.Errorf("ClampDuration(%g, %g, %g) = %g, want %g",
					tc.value, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		v    any
		want float64
	}{
		{"float64", float64(5.5), 5.5},
		{"int", int(3), 3.0},
		{"float32", float32(2.5), 2.5},
		{"string", "nope", 0},
		{"nil", nil, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := toFloat64(tc.v)
			if got != tc.want {
				t.Errorf("toFloat64(%v) = %g, want %g", tc.v, got, tc.want)
			}
		})
	}
}

func TestExtractDuration_Float32(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": float32(4.5)}},
	}
	d, ok := ExtractDuration(g)
	if !ok || d != 4.5 {
		t.Errorf("ExtractDuration(float32) = (%g, %v), want (4.5, true)", d, ok)
	}
}

func TestExtractDuration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		graph workflow.Graph
		want  float64
		wantOK bool
	}{
		{
			"from VideoOptions int",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 5}},
			},
			5, true,
		},
		{
			"from VideoOptions float64",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 7.5}},
			},
			7.5, true,
		},
		{
			"from global option",
			workflow.Graph{
				"1": {ClassType: "SomeNode", Inputs: map[string]any{"duration": 10}},
			},
			10, true,
		},
		{
			"VideoOptions takes priority",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 3}},
				"2": {ClassType: "Other", Inputs: map[string]any{"duration": 99}},
			},
			3, true,
		},
		{
			"empty graph",
			workflow.Graph{},
			0, false,
		},
		{
			"zero duration ignored",
			workflow.Graph{
				"1": {ClassType: "VideoOptions", Inputs: map[string]any{"duration": 0}},
			},
			0, false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ExtractDuration(tc.graph)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("ExtractDuration = %g, want %g", got, tc.want)
			}
		})
	}
}
