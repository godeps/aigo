package engine

import (
	"fmt"
	"os"
	"strings"
)

// ResolveKey resolves an API key by checking the explicit value first,
// then falling back to the given environment variable names in order.
// Returns an error listing all checked sources if no key is found.
func ResolveKey(explicit string, envVars ...string) (string, error) {
	if k := strings.TrimSpace(explicit); k != "" {
		return k, nil
	}
	for _, ev := range envVars {
		if k := strings.TrimSpace(os.Getenv(ev)); k != "" {
			return k, nil
		}
	}
	if len(envVars) == 0 {
		return "", fmt.Errorf("missing API key")
	}
	return "", fmt.Errorf("missing API key (set config or env %s)", strings.Join(envVars, " / "))
}

// ResolveKeyPair resolves a pair of keys (e.g. AccessKey + SecretKey).
// Each key is resolved independently via ResolveKey.
func ResolveKeyPair(ak, sk string, akEnvs, skEnvs []string) (string, string, error) {
	a, err := ResolveKey(ak, akEnvs...)
	if err != nil {
		return "", "", fmt.Errorf("access key: %w", err)
	}
	s, err := ResolveKey(sk, skEnvs...)
	if err != nil {
		return "", "", fmt.Errorf("secret key: %w", err)
	}
	return a, s, nil
}
