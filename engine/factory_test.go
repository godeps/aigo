package engine

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEngineConfig_WaitForCompletionRoundTrip pins the JSON wire shape so
// users editing settings.json can rely on the field name being stable.
func TestEngineConfig_WaitForCompletionRoundTrip(t *testing.T) {
	t.Parallel()

	tt := true
	ff := false
	cases := []struct {
		name string
		raw  string
		want *bool
	}{
		{"explicit true", `{"name":"x","provider":"y","wait_for_completion":true}`, &tt},
		{"explicit false", `{"name":"x","provider":"y","wait_for_completion":false}`, &ff},
		{"absent", `{"name":"x","provider":"y"}`, nil},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var cfg EngineConfig
			if err := json.Unmarshal([]byte(c.raw), &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			switch {
			case c.want == nil && cfg.WaitForCompletion != nil:
				t.Fatalf("expected nil, got %v", *cfg.WaitForCompletion)
			case c.want != nil && cfg.WaitForCompletion == nil:
				t.Fatalf("expected %v, got nil", *c.want)
			case c.want != nil && *c.want != *cfg.WaitForCompletion:
				t.Fatalf("expected %v, got %v", *c.want, *cfg.WaitForCompletion)
			}
		})
	}
}

func TestEngineConfig_WaitForCompletionOr(t *testing.T) {
	t.Parallel()
	tt := true
	ff := false
	cases := []struct {
		name string
		ptr  *bool
		def  bool
		want bool
	}{
		{"unset uses default true", nil, true, true},
		{"unset uses default false", nil, false, false},
		{"explicit true overrides false default", &tt, false, true},
		{"explicit false overrides true default", &ff, true, false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg := EngineConfig{WaitForCompletion: c.ptr}
			if got := cfg.WaitForCompletionOr(c.def); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestEngineConfig_PollIntervalRoundTrip(t *testing.T) {
	t.Parallel()
	cfg := EngineConfig{Name: "x", Provider: "y", PollInterval: 2 * time.Second}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back EngineConfig
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.PollInterval != 2*time.Second {
		t.Fatalf("got %v, want 2s", back.PollInterval)
	}
}
