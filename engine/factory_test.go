package engine

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/godeps/aigo/engine/httpx"
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

func TestEngineConfig_QualityRoundTrip(t *testing.T) {
	t.Parallel()

	raw := `{"name":"x","provider":"newapi","model":"gpt-image-1-mini","quality":"high"}`
	var cfg EngineConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Quality != "high" {
		t.Fatalf("quality = %q, want high", cfg.Quality)
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"quality":"high"`) {
		t.Fatalf("marshaled config missing quality: %s", out)
	}
}

type factoryTestRequestHook struct{}

func (factoryTestRequestHook) BeforeRequest(req *http.Request) (*http.Request, error) {
	return req, nil
}

func TestEngineConfig_ClientWithHooksAppendsHooks(t *testing.T) {
	t.Parallel()

	base := httpx.WithHTTPHooks(nil, httpx.WithRequestHooks(factoryTestRequestHook{}))
	cfg := EngineConfig{
		HTTPClient:      base,
		HTTPHookOptions: []httpx.HookOption{httpx.WithRequestHooks(factoryTestRequestHook{})},
	}

	client := cfg.ClientWithHooks()
	transport, ok := client.Transport.(*httpx.HookTransport)
	if !ok {
		t.Fatalf("transport = %T, want *httpx.HookTransport", client.Transport)
	}
	if _, nested := transport.Base.(*httpx.HookTransport); nested {
		t.Fatal("ClientWithHooks should append hooks, not nest HookTransport")
	}
	if len(transport.RequestHooks) != 2 {
		t.Fatalf("request hook count = %d, want 2", len(transport.RequestHooks))
	}
}
