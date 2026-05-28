package newapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/godeps/aigo/engine/newapi/internal/rt"
)

const discoverTimeout = 5 * time.Second

// ModelEntry represents a single model from the /v1/models response.
// Compatible with both standard OpenAI format and new-api extended format.
type ModelEntry struct {
	ID                     string   `json:"id"`
	Object                 string   `json:"object"`
	OwnedBy               string   `json:"owned_by,omitempty"`
	Capability             string   `json:"capability,omitempty"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types,omitempty"`
}

// modelsResponse is the OpenAI-compatible /v1/models response envelope.
type modelsResponse struct {
	Object string       `json:"object"`
	Data   []ModelEntry `json:"data"`
}

// DiscoverModels calls GET /v1/models on the given gateway and returns models
// grouped by capability. For each model, capability is determined via:
//  1. The "capability" field in the API response (if the gateway provides it)
//  2. knownModels exact match
//  3. Model name pattern inference
//
// Models that cannot be classified are placed under the "unknown" key.
func DiscoverModels(ctx context.Context, baseURL, apiKey string) (map[string][]string, error) {
	origin := rt.NormalizeOrigin(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if origin == "" {
		return nil, fmt.Errorf("newapi: discover: baseURL is empty")
	}

	url := rt.Join(origin, "/v1/models")

	ctx, cancel := context.WithTimeout(ctx, discoverTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("newapi: discover: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("newapi: discover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("newapi: discover: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var models modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("newapi: discover: decode: %w", err)
	}

	result := make(map[string][]string)
	for _, m := range models.Data {
		cap := classifyModel(m)
		result[cap] = append(result[cap], m.ID)
	}
	return result, nil
}

// classifyModel determines the capability of a model entry.
func classifyModel(m ModelEntry) string {
	// Priority 1: explicit capability field from API response
	if m.Capability != "" {
		return m.Capability
	}

	// Priority 2: supported_endpoint_types (new-api format)
	if cap := capFromEndpointTypes(m.SupportedEndpointTypes); cap != "" {
		return cap
	}

	// Priority 3: knownModels catalog
	if entry, ok := knownModels[m.ID]; ok {
		return entry.cap
	}

	// Priority 4: model name inference
	if _, _, cap := InferFromModelName(m.ID); cap != "" {
		return cap
	}

	return "unknown"
}

// endpointTypeCapMap maps new-api supported_endpoint_types to aigo capabilities.
var endpointTypeCapMap = map[string]string{
	"image-generation": "image",
	"openai-video":     "video",
}

// capFromEndpointTypes extracts aigo capability from new-api endpoint types.
// Returns the first matching media-generation capability found.
func capFromEndpointTypes(types []string) string {
	for _, t := range types {
		if cap, ok := endpointTypeCapMap[t]; ok {
			return cap
		}
	}
	return ""
}
