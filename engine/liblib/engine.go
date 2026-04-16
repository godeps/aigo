// Package liblib implements engine.Engine for the LibLibAI open API.
//
// LibLib uses HMAC-SHA1 signature authentication with AccessKey/SecretKey.
// Generation is async: POST a submit endpoint → poll POST /api/generate/webui/status
// until generateStatus == 5 (success) or 6/7 (failure).
// Base URL: https://openapi.liblibai.cloud
package liblib

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/godeps/aigo/engine"
	"github.com/godeps/aigo/engine/aigoerr"
	"github.com/godeps/aigo/engine/httpx"
	epoll "github.com/godeps/aigo/engine/poll"
	"github.com/godeps/aigo/workflow"
	"github.com/godeps/aigo/workflow/resolve"
)

const (
	defaultBaseURL      = "https://openapi.liblibai.cloud"
	defaultPollInterval = 5 * time.Second
)

// Well-known templateUUIDs for built-in endpoints.
const (
	TemplateText2ImgUltra = "5d7e67009b344550bc1aa6ccbfa1d7f4"
	TemplateText2Img      = "e10adc3949ba59abbe56e057f20f883e"
	TemplateImg2ImgUltra  = "07e00af4fc464c7ab55ff906f8acf1b7"
	TemplateImg2Img       = "9c7d531dc75f476aa833b3d452b8f7ad"
	TemplateKontext       = "fe9928fde1b4491c9b360dd24aa2b115"
	TemplateKlingText2Vid = "61cd8b60d340404394f2a545eeaf197a"
	TemplateKlingImg2Vid  = "180f33c6748041b48593030156d2a71d"
)

var (
	ErrMissingAccessKey = errors.New("liblib: missing AccessKey (set Config.AccessKey or LIBLIB_ACCESS_KEY)")
	ErrMissingSecretKey = errors.New("liblib: missing SecretKey (set Config.SecretKey or LIBLIB_SECRET_KEY)")
	ErrMissingPrompt    = errors.New("liblib: missing prompt in workflow graph")
)

// Config configures the LibLib engine.
type Config struct {
	AccessKey         string
	SecretKey         string
	BaseURL           string        // default: https://openapi.liblibai.cloud
	Endpoint          string        // submit URI path, e.g. "/api/generate/webui/text2img/ultra"
	TemplateUUID      string        // required template UUID for the generation type
	HTTPClient        *http.Client
	WaitForCompletion bool
	PollInterval      time.Duration // default: 5s
}

// Engine implements engine.Engine for LibLib.
type Engine struct {
	accessKey    string
	secretKey    string
	baseURL      string
	endpoint     string
	templateUUID string
	httpClient   *http.Client
	waitResult   bool
	pollInterval time.Duration
}

// New creates a LibLib engine instance.
func New(cfg Config) *Engine {
	hc := httpx.OrDefault(cfg.HTTPClient, httpx.DefaultTimeout)

	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("LIBLIB_BASE_URL")), "/")
	}
	if base == "" {
		base = defaultBaseURL
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "/api/generate/webui/text2img/ultra"
	}

	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}

	return &Engine{
		accessKey:    strings.TrimSpace(cfg.AccessKey),
		secretKey:    strings.TrimSpace(cfg.SecretKey),
		baseURL:      base,
		endpoint:     endpoint,
		templateUUID: strings.TrimSpace(cfg.TemplateUUID),
		httpClient:   hc,
		waitResult:   cfg.WaitForCompletion,
		pollInterval: poll,
	}
}

// Execute generates content via the LibLib API.
func (e *Engine) Execute(ctx context.Context, g workflow.Graph) (engine.Result, error) {
	if err := g.Validate(); err != nil {
		return engine.Result{}, fmt.Errorf("liblib: validate graph: %w", err)
	}

	ak, sk, err := e.resolveKeys()
	if err != nil {
		return engine.Result{}, err
	}

	prompt, err := resolve.ExtractPrompt(g)
	if err != nil {
		return engine.Result{}, fmt.Errorf("liblib: %w", err)
	}
	if prompt == "" {
		return engine.Result{}, ErrMissingPrompt
	}

	params := map[string]any{
		"prompt": prompt,
	}

	// Extract image reference for img2img or video.
	for _, ref := range g.FindByClassType("LoadImage") {
		if u, ok := ref.Node.Inputs["url"].(string); ok && u != "" {
			params["sourceImage"] = u
			break
		}
	}

	// Extract common options from graph.
	if w, ok := resolve.IntOption(g, "width"); ok && w > 0 {
		if h, ok2 := resolve.IntOption(g, "height"); ok2 && h > 0 {
			params["imageSize"] = map[string]any{"width": w, "height": h}
		}
	}
	if ar, ok := resolve.StringOption(g, "aspect_ratio", "aspectRatio"); ok && ar != "" {
		params["aspectRatio"] = ar
	}
	if n, ok := resolve.IntOption(g, "img_count", "imgCount"); ok && n > 0 {
		params["imgCount"] = n
	}
	if d, ok := resolve.IntOption(g, "duration"); ok && d > 0 {
		params["duration"] = d
	}
	if np, ok := resolve.StringOption(g, "negative_prompt", "negativePrompt"); ok && np != "" {
		params["negativePrompt"] = np
	}
	if m, ok := resolve.StringOption(g, "model"); ok && m != "" {
		params["model"] = m
	}

	// Merge any extra JSON params.
	resolve.MergeJSONOption(g, params, "extra", "extra_params")

	payload := map[string]any{
		"generateParams": params,
	}
	if e.templateUUID != "" {
		payload["templateUuid"] = e.templateUUID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return engine.Result{}, fmt.Errorf("liblib: marshal request: %w", err)
	}

	respBody, err := e.doSignedRequest(ctx, ak, sk, e.endpoint, body)
	if err != nil {
		return engine.Result{}, err
	}

	var resp apiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return engine.Result{}, fmt.Errorf("liblib: decode submit response: %w", err)
	}
	if resp.Code != 0 {
		return engine.Result{}, fmt.Errorf("liblib: API error %d: %s", resp.Code, resp.Msg)
	}

	generateUUID, _ := resp.Data["generateUuid"].(string)
	if generateUUID == "" {
		return engine.Result{}, fmt.Errorf("liblib: submit returned empty generateUuid")
	}

	if !e.waitResult {
		return engine.Result{Value: generateUUID, Kind: engine.OutputPlainText}, nil
	}

	resultURL, err := e.poll(ctx, ak, sk, generateUUID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: resultURL, Kind: engine.OutputURL}, nil
}

// Resume implements engine.Resumer — resumes polling a previously submitted task.
func (e *Engine) Resume(ctx context.Context, remoteID string) (engine.Result, error) {
	ak, sk, err := e.resolveKeys()
	if err != nil {
		return engine.Result{}, err
	}
	url, err := e.poll(ctx, ak, sk, remoteID)
	if err != nil {
		return engine.Result{}, err
	}
	return engine.Result{Value: url, Kind: engine.OutputURL}, nil
}

// Capabilities implements engine.Describer.
func (e *Engine) Capabilities() engine.Capability {
	return engine.Capability{
		MediaTypes:   []string{"image", "video"},
		SupportsPoll: e.waitResult,
		SupportsSync: !e.waitResult,
	}
}

// ConfigSchema returns the configuration fields for the LibLib engine.
func ConfigSchema() []engine.ConfigField {
	return []engine.ConfigField{
		{Key: "accessKey", Label: "Access Key", Type: "secret", Required: true, EnvVar: "LIBLIB_ACCESS_KEY", Description: "LibLib API access key"},
		{Key: "secretKey", Label: "Secret Key", Type: "secret", Required: true, EnvVar: "LIBLIB_SECRET_KEY", Description: "LibLib API secret key"},
		{Key: "baseUrl", Label: "Base URL", Type: "url", EnvVar: "LIBLIB_BASE_URL", Description: "Custom API base URL (optional)", Default: defaultBaseURL},
		{Key: "endpoint", Label: "Endpoint", Type: "string", Required: true, Description: "API endpoint path, e.g. /api/generate/webui/text2img/ultra"},
		{Key: "templateUuid", Label: "Template UUID", Type: "string", Required: true, Description: "Template UUID for the generation type"},
	}
}

// ModelsByCapability returns known LibLib model capabilities.
func ModelsByCapability() map[string][]string {
	return map[string][]string{
		"image": {"text2img", "text2img/ultra", "img2img", "img2img/ultra", "kontext"},
		"video": {"kling-v2-master", "kling-v2-1-master", "kling-v1-6", "kling-v2-1"},
	}
}

// --- internal ---

type apiResponse struct {
	Code int            `json:"code"`
	Msg  string         `json:"msg"`
	Data map[string]any `json:"data"`
}

func (e *Engine) resolveKeys() (string, string, error) {
	ak := e.accessKey
	if ak == "" {
		ak = os.Getenv("LIBLIB_ACCESS_KEY")
	}
	if ak == "" {
		return "", "", ErrMissingAccessKey
	}
	sk := e.secretKey
	if sk == "" {
		sk = os.Getenv("LIBLIB_SECRET_KEY")
	}
	if sk == "" {
		return "", "", ErrMissingSecretKey
	}
	return ak, sk, nil
}

// signURL builds a signed URL with HMAC-SHA1 authentication.
func (e *Engine) signURL(ak, sk, uri string) string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	nonce := randomHex()

	content := uri + "&" + ts + "&" + nonce
	mac := hmac.New(sha1.New, []byte(sk))
	mac.Write([]byte(content))
	sig := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s%s?AccessKey=%s&Signature=%s&Timestamp=%s&SignatureNonce=%s",
		e.baseURL, uri, ak, sig, ts, nonce)
}

// randomHex generates a random 32-character hex string.
func randomHex() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("liblib: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func (e *Engine) doSignedRequest(ctx context.Context, ak, sk, uri string, body []byte) ([]byte, error) {
	url := e.signURL(ak, sk, uri)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("liblib: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("liblib: http POST: %w", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("liblib: read body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, aigoerr.FromHTTPResponse(resp, out, "liblib")
	}
	return out, nil
}

func (e *Engine) poll(ctx context.Context, ak, sk, generateUUID string) (string, error) {
	pollBody, _ := json.Marshal(map[string]string{"generateUuid": generateUUID})

	return epoll.Poll(ctx, epoll.Config{Interval: e.pollInterval}, func(ctx context.Context) (string, bool, error) {
		respBody, err := e.doSignedRequest(ctx, ak, sk, "/api/generate/webui/status", pollBody)
		if err != nil {
			return "", false, err
		}

		var resp apiResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return "", false, fmt.Errorf("liblib: decode poll: %w", err)
		}
		if resp.Code != 0 {
			return "", true, fmt.Errorf("liblib: poll error %d: %s", resp.Code, resp.Msg)
		}

		status, _ := resp.Data["generateStatus"].(float64)
		switch int(status) {
		case 5: // success
			// Try images first, then videos.
			if images, ok := resp.Data["images"].([]any); ok && len(images) > 0 {
				if img, ok := images[0].(map[string]any); ok {
					if u, ok := img["imageUrl"].(string); ok && u != "" {
						return u, true, nil
					}
				}
			}
			if videos, ok := resp.Data["videos"].([]any); ok && len(videos) > 0 {
				if vid, ok := videos[0].(map[string]any); ok {
					if u, ok := vid["videoUrl"].(string); ok && u != "" {
						return u, true, nil
					}
				}
			}
			return "", true, fmt.Errorf("liblib: succeeded but no output URL found")
		case 6, 7: // failed / error
			return "", true, fmt.Errorf("liblib: generation failed (status %d)", int(status))
		default:
			return "", false, nil // still running
		}
	})
}
