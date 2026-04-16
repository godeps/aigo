package engine

import (
	"strings"
	"testing"
)

func TestResolveKey_Explicit(t *testing.T) {
	t.Parallel()
	k, err := ResolveKey("my-key", "UNLIKELY_ENV_VAR_XYZ")
	if err != nil {
		t.Fatal(err)
	}
	if k != "my-key" {
		t.Errorf("got %q, want %q", k, "my-key")
	}
}

func TestResolveKey_EnvFallback(t *testing.T) {
	t.Setenv("TEST_AUTH_KEY_1", "env-key")
	k, err := ResolveKey("", "TEST_AUTH_KEY_1")
	if err != nil {
		t.Fatal(err)
	}
	if k != "env-key" {
		t.Errorf("got %q, want %q", k, "env-key")
	}
}

func TestResolveKey_MultipleEnvs(t *testing.T) {
	t.Setenv("TEST_AUTH_KEY_B", "second")
	k, err := ResolveKey("", "TEST_AUTH_KEY_A", "TEST_AUTH_KEY_B")
	if err != nil {
		t.Fatal(err)
	}
	if k != "second" {
		t.Errorf("got %q, want %q", k, "second")
	}
}

func TestResolveKey_Missing(t *testing.T) {
	t.Parallel()
	_, err := ResolveKey("", "NO_SUCH_VAR_1", "NO_SUCH_VAR_2")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NO_SUCH_VAR_1") {
		t.Errorf("error should mention env vars: %v", err)
	}
}

func TestResolveKey_Trimmed(t *testing.T) {
	t.Parallel()
	k, err := ResolveKey("  spaced  ")
	if err != nil {
		t.Fatal(err)
	}
	if k != "spaced" {
		t.Errorf("got %q, want %q", k, "spaced")
	}
}

func TestResolveKeyPair_Success(t *testing.T) {
	t.Parallel()
	a, s, err := ResolveKeyPair("ak", "sk", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if a != "ak" || s != "sk" {
		t.Errorf("got (%q, %q)", a, s)
	}
}

func TestResolveKeyPair_MissingSecret(t *testing.T) {
	t.Parallel()
	_, _, err := ResolveKeyPair("ak", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing secret key")
	}
	if !strings.Contains(err.Error(), "secret key") {
		t.Errorf("error should mention secret key: %v", err)
	}
}
