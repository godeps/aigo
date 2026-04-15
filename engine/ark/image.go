package ark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/godeps/aigo/workflow"
)

const imagesPath = "/api/v3/images/generations"

// Seedream model constants.
const (
	ModelSeedream3_0 = "seedream-3.0"
	ModelSeedream2_1 = "seedream-2.1"
)

// runImageGeneration generates an image via the Ark /images/generations endpoint.
//
// Request format (OpenAI-compatible):
//
//	{
//	  "model": "seedream-3.0",
//	  "prompt": "<prompt>",
//	  "response_format": "url",
//	  "size": "1024x1024"
//	}
//
// Response: {"data": [{"url": "https://..."} | {"b64_json": "..."}]}
func runImageGeneration(ctx context.Context, e *Engine, apiKey string, g workflow.Graph) (string, error) {
	prompt := extractPrompt(g)
	if prompt == "" {
		return "", ErrMissingContent
	}

	payload := map[string]any{
		"model":  e.model,
		"prompt": prompt,
	}

	// Response format: "url" (default) or "b64_json".
	respFmt := "url"
	if v, ok := stringOption(g, "response_format"); ok {
		respFmt = v
	}
	payload["response_format"] = respFmt

	if v, ok := stringOption(g, "size"); ok {
		payload["size"] = v
	}
	if v, ok := intOption(g, "seed"); ok {
		payload["seed"] = v
	}
	if v, ok := boolOption(g, "watermark"); ok {
		payload["watermark"] = v
	}
	if v, ok := boolOption(g, "optimize_prompt"); ok {
		payload["optimize_prompt"] = v
	}
	// Reference image for editing.
	if v, ok := stringOption(g, "image", "image_url"); ok {
		payload["image"] = v
	}
	if v, ok := stringOption(g, "guidance_scale"); ok {
		payload["guidance_scale"] = v
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ark: marshal image request: %w", err)
	}

	respBody, err := e.doRequest(ctx, http.MethodPost, e.baseURL+imagesPath, apiKey, body)
	if err != nil {
		return "", err
	}
	return extractImageResult(respBody, respFmt)
}

// extractImageResult parses the /images/generations response.
func extractImageResult(body []byte, format string) (string, error) {
	var resp struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
		Error *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("ark: decode image response: %w", err)
	}
	if resp.Error != nil && resp.Error.Message != "" {
		return "", fmt.Errorf("ark: image api error %s: %s", resp.Error.Code, resp.Error.Message)
	}
	if len(resp.Data) == 0 {
		return "", fmt.Errorf("ark: image response had no data")
	}

	img := resp.Data[0]
	if strings.EqualFold(format, "b64_json") && img.B64JSON != "" {
		return "data:image/png;base64," + img.B64JSON, nil
	}
	if img.URL != "" {
		return img.URL, nil
	}
	if img.B64JSON != "" {
		return "data:image/png;base64," + img.B64JSON, nil
	}
	return "", fmt.Errorf("ark: image response had no url or b64_json")
}
