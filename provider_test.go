package aigo

import (
	"os"
	"testing"

	"github.com/godeps/aigo/engine"

	// Blank imports to trigger init() factory registrations.
	_ "github.com/godeps/aigo/engine/alibabacloud"
	_ "github.com/godeps/aigo/engine/kling"
	_ "github.com/godeps/aigo/engine/luma"
)

func TestRegisterAll(t *testing.T) {
	t.Parallel()

	client := NewClient()
	engines := map[string]engine.Engine{
		"a": stubEngine{result: "a"},
		"b": stubEngine{result: "b"},
	}

	if err := client.RegisterAll(engines); err != nil {
		t.Fatalf("RegisterAll() error = %v", err)
	}

	names := client.EngineNames()
	if len(names) != 2 {
		t.Fatalf("RegisterAll() registered %d engines, want 2", len(names))
	}
}

func TestRegisterAllRejectsDuplicate(t *testing.T) {
	t.Parallel()

	client := NewClient()
	_ = client.RegisterEngine("dup", stubEngine{})

	err := client.RegisterAll(map[string]engine.Engine{
		"dup": stubEngine{},
	})
	if err == nil {
		t.Fatal("RegisterAll() expected error for duplicate")
	}
}

func TestRegisterAllIfKey(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	t.Setenv("TEST_AIGO_KEY_PRESENT", "xxx")

	client := NewClient()
	entries := []EngineEntry{
		{Name: "has-key", Engine: stubEngine{result: "yes"}, EnvVars: []string{"TEST_AIGO_KEY_PRESENT"}},
		{Name: "no-key", Engine: stubEngine{result: "no"}, EnvVars: []string{"TEST_AIGO_KEY_ABSENT_" + t.Name()}},
		{Name: "always", Engine: stubEngine{result: "always"}},
	}

	registered, err := client.RegisterAllIfKey(entries)
	if err != nil {
		t.Fatalf("RegisterAllIfKey() error = %v", err)
	}

	if len(registered) != 2 {
		t.Fatalf("RegisterAllIfKey() registered %d, want 2: %v", len(registered), registered)
	}
	if registered[0] != "has-key" || registered[1] != "always" {
		t.Errorf("registered = %v, want [has-key, always]", registered)
	}
}

func TestRegisterProvider(t *testing.T) {
	t.Setenv("TEST_PROVIDER_KEY", "set")

	client := NewClient()
	p := engine.Provider{
		Name: "test-vendor",
		Configs: []engine.ProviderConfig{
			{
				Name:    "test-vendor-a",
				Engine:  stubEngine{result: "a"},
				EnvVars: []string{"TEST_PROVIDER_KEY"},
			},
			{
				Name:    "test-vendor-b",
				Engine:  stubEngine{result: "b"},
				EnvVars: []string{"TEST_PROVIDER_MISSING_" + t.Name()},
			},
			{
				Name:   "test-vendor-c",
				Engine: stubEngine{result: "c"},
			},
		},
	}

	registered, err := client.RegisterProvider(p)
	if err != nil {
		t.Fatalf("RegisterProvider() error = %v", err)
	}
	if len(registered) != 2 {
		t.Fatalf("RegisterProvider() registered %d, want 2: %v", len(registered), registered)
	}
	if registered[0] != "test-vendor-a" || registered[1] != "test-vendor-c" {
		t.Errorf("registered = %v, want [test-vendor-a, test-vendor-c]", registered)
	}
}

func TestRegisteredFactories(t *testing.T) {
	t.Parallel()

	factories := engine.RegisteredFactories()
	if len(factories) == 0 {
		t.Fatal("RegisteredFactories() returned empty, expected at least some registered factories")
	}

	// Verify well-known providers are registered via init().
	known := map[string]bool{
		"alibabacloud": false,
		"kling":        false,
		"luma":         false,
	}
	for _, f := range factories {
		if _, ok := known[f]; ok {
			known[f] = true
		}
	}
	for k, found := range known {
		if !found {
			t.Errorf("expected factory %q to be registered", k)
		}
	}
}

func TestDefaultProviderEnvGating(t *testing.T) {
	orig := os.Getenv("KLING_API_KEY")
	os.Unsetenv("KLING_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("KLING_API_KEY", orig)
		}
	}()

	factory, ok := engine.GetFactory("kling")
	if !ok {
		t.Fatal("expected kling factory to be registered")
	}

	eng, err := factory(engine.EngineConfig{Model: "test"})
	if err != nil {
		t.Fatalf("kling factory error: %v", err)
	}
	if eng == nil {
		t.Fatal("kling factory returned nil engine")
	}
}
