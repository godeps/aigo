package engine

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseURI_APIKeyOnly(t *testing.T) {
	cfg, err := ParseURI("dashscope://sk-abc123")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "alibabacloud" {
		t.Errorf("Provider = %q, want alibabacloud", cfg.Provider)
	}
	if cfg.APIKey != "sk-abc123" {
		t.Errorf("APIKey = %q, want sk-abc123", cfg.APIKey)
	}
	if cfg.BaseURL != "" {
		t.Errorf("BaseURL = %q, want empty", cfg.BaseURL)
	}
	if cfg.Name != "alibabacloud" {
		t.Errorf("Name = %q, want alibabacloud", cfg.Name)
	}
}

func TestParseURI_HostBaseURL(t *testing.T) {
	cfg, err := ParseURI("dashscope://sk-xxx@my-proxy.com/api/v1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "alibabacloud" {
		t.Errorf("Provider = %q, want alibabacloud", cfg.Provider)
	}
	if cfg.APIKey != "sk-xxx" {
		t.Errorf("APIKey = %q, want sk-xxx", cfg.APIKey)
	}
	if cfg.BaseURL != "https://my-proxy.com/api/v1" {
		t.Errorf("BaseURL = %q, want https://my-proxy.com/api/v1", cfg.BaseURL)
	}
}

func TestParseURI_QueryBaseURL(t *testing.T) {
	cfg, err := ParseURI("openai://sk-abc?base_url=https://custom.com/v1&model=dall-e-3")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", cfg.Provider)
	}
	if cfg.APIKey != "sk-abc" {
		t.Errorf("APIKey = %q, want sk-abc", cfg.APIKey)
	}
	if cfg.BaseURL != "https://custom.com/v1" {
		t.Errorf("BaseURL = %q, want https://custom.com/v1", cfg.BaseURL)
	}
	if cfg.Model != "dall-e-3" {
		t.Errorf("Model = %q, want dall-e-3", cfg.Model)
	}
}

func TestParseURI_QueryOverridesHost(t *testing.T) {
	cfg, err := ParseURI("dashscope://sk-xxx@default-host.com/v1?base_url=https://override.com/api")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://override.com/api" {
		t.Errorf("BaseURL = %q, want https://override.com/api (query should override host)", cfg.BaseURL)
	}
}

func TestParseURI_FullParams(t *testing.T) {
	cfg, err := ParseURI("dashscope://sk-xxx@host.com/path?model=qwen-image&wait=true&poll_interval=5s&capability=image&name=my-dashscope&voice_id=cherry")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "alibabacloud" {
		t.Errorf("Provider = %q", cfg.Provider)
	}
	if cfg.APIKey != "sk-xxx" {
		t.Errorf("APIKey = %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://host.com/path" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Model != "qwen-image" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.WaitForCompletion == nil || !*cfg.WaitForCompletion {
		t.Error("WaitForCompletion should be true")
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.Capability != "image" {
		t.Errorf("Capability = %q", cfg.Capability)
	}
	if cfg.Name != "my-dashscope" {
		t.Errorf("Name = %q, want my-dashscope", cfg.Name)
	}
	if cfg.Metadata == nil || cfg.Metadata["voice_id"] != "cherry" {
		t.Errorf("Metadata[voice_id] = %q, want cherry", cfg.Metadata["voice_id"])
	}
}

func TestParseURI_WaitFalse(t *testing.T) {
	cfg, err := ParseURI("kling://key123?wait=false")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WaitForCompletion == nil || *cfg.WaitForCompletion {
		t.Error("WaitForCompletion should be false")
	}
}

func TestParseURIs_Multiple(t *testing.T) {
	cfgs, err := ParseURIs("dashscope://sk-1?model=qwen-image,openai://sk-2?model=dall-e-3")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("got %d configs, want 2", len(cfgs))
	}
	if cfgs[0].Provider != "alibabacloud" || cfgs[0].APIKey != "sk-1" || cfgs[0].Model != "qwen-image" {
		t.Errorf("cfgs[0] = %+v", cfgs[0])
	}
	if cfgs[1].Provider != "openai" || cfgs[1].APIKey != "sk-2" || cfgs[1].Model != "dall-e-3" {
		t.Errorf("cfgs[1] = %+v", cfgs[1])
	}
}

func TestParseURI_Errors(t *testing.T) {
	cases := []struct {
		name string
		uri  string
	}{
		{"empty", ""},
		{"no scheme", "sk-abc123"},
		{"missing key", "dashscope://"},
		{"missing key with host", "dashscope://@host.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseURI(tc.uri)
			if err == nil {
				t.Errorf("ParseURI(%q) should return error", tc.uri)
			}
		})
	}
}

func TestParseURIs_Empty(t *testing.T) {
	_, err := ParseURIs("")
	if err == nil {
		t.Error("ParseURIs(\"\") should return error")
	}
}

func TestParseURI_CaseInsensitiveScheme(t *testing.T) {
	cfg, err := ParseURI("DashScope://sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "alibabacloud" {
		t.Errorf("Provider = %q, want alibabacloud (resolved alias)", cfg.Provider)
	}
}

func TestParseURI_SchemeAlias(t *testing.T) {
	cases := []struct {
		uri      string
		provider string
	}{
		{"dashscope://sk-xxx", "alibabacloud"},
		{"tongyi://sk-xxx", "alibabacloud"},
		{"bailian://sk-xxx", "alibabacloud"},
		{"doubao://sk-xxx", "ark"},
		{"volcengine://sk-xxx", "ark"},
		{"dall-e://sk-xxx", "openai"},
		{"dalle://sk-xxx", "openai"},
	}
	for _, tc := range cases {
		t.Run(tc.uri, func(t *testing.T) {
			cfg, err := ParseURI(tc.uri)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Provider != tc.provider {
				t.Errorf("Provider = %q, want %q", cfg.Provider, tc.provider)
			}
		})
	}
}

func TestRegisterSchemeAlias(t *testing.T) {
	RegisterSchemeAlias("mycloud", "alibabacloud")
	cfg, err := ParseURI("mycloud://sk-test?model=test-model")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Provider != "alibabacloud" {
		t.Errorf("Provider = %q, want alibabacloud", cfg.Provider)
	}
	if cfg.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", cfg.Model)
	}
}

// --- 优化: AK:SK 双密钥 ---

func TestParseURI_DualKey_WithHost(t *testing.T) {
	cfg, err := ParseURI("kling://ak-123:sk-456@api.klingai.com/v1?model=kling-v3")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "ak-123" {
		t.Errorf("APIKey = %q, want ak-123", cfg.APIKey)
	}
	if cfg.Metadata == nil || cfg.Metadata["secretKey"] != "sk-456" {
		t.Errorf("Metadata[secretKey] = %q, want sk-456", cfg.Metadata["secretKey"])
	}
	if cfg.BaseURL != "https://api.klingai.com/v1" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Model != "kling-v3" {
		t.Errorf("Model = %q", cfg.Model)
	}
}

func TestParseURI_DualKey_NoHost(t *testing.T) {
	cfg, err := ParseURI("jimeng://ak-111:sk-222?model=jimeng-model")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKey != "ak-111" {
		t.Errorf("APIKey = %q, want ak-111", cfg.APIKey)
	}
	if cfg.Metadata == nil || cfg.Metadata["secretKey"] != "sk-222" {
		t.Errorf("Metadata[secretKey] = %q, want sk-222", cfg.Metadata["secretKey"])
	}
	if cfg.BaseURL != "" {
		t.Errorf("BaseURL = %q, want empty", cfg.BaseURL)
	}
}

// --- 优化: http 协议支持 ---

func TestParseURI_LocalhostHTTP(t *testing.T) {
	cfg, err := ParseURI("openai://sk-test@localhost:8080/v1?model=dall-e-3")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want http://localhost:8080/v1", cfg.BaseURL)
	}
}

func TestParseURI_127001HTTP(t *testing.T) {
	cfg, err := ParseURI("dashscope://sk-test@127.0.0.1:3000/api")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://127.0.0.1:3000/api" {
		t.Errorf("BaseURL = %q, want http://127.0.0.1:3000/api", cfg.BaseURL)
	}
}

func TestParseURI_SchemeQueryHTTP(t *testing.T) {
	cfg, err := ParseURI("openai://sk-test@my-proxy.com/v1?scheme=http")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "http://my-proxy.com/v1" {
		t.Errorf("BaseURL = %q, want http://my-proxy.com/v1", cfg.BaseURL)
	}
}

func TestParseURI_SchemeQueryHTTPS(t *testing.T) {
	cfg, err := ParseURI("openai://sk-test@localhost:8080/v1?scheme=https")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want https://localhost:8080/v1 (explicit https overrides localhost default)", cfg.BaseURL)
	}
}

// --- 优化: Name 自动去重 ---

func TestParseURIs_DuplicateNames(t *testing.T) {
	cfgs, err := ParseURIs("dashscope://sk-1?model=qwen-image,dashscope://sk-2?model=wan-video")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 2 {
		t.Fatalf("got %d configs, want 2", len(cfgs))
	}
	if cfgs[0].Name != "alibabacloud-qwen-image" {
		t.Errorf("cfgs[0].Name = %q, want alibabacloud-qwen-image", cfgs[0].Name)
	}
	if cfgs[1].Name != "alibabacloud-wan-video" {
		t.Errorf("cfgs[1].Name = %q, want alibabacloud-wan-video", cfgs[1].Name)
	}
}

func TestParseURIs_DuplicateNamesSameModel(t *testing.T) {
	cfgs, err := ParseURIs("dashscope://sk-1?model=qwen-image,dashscope://sk-2?model=qwen-image")
	if err != nil {
		t.Fatal(err)
	}
	if cfgs[0].Name != "alibabacloud-qwen-image" {
		t.Errorf("cfgs[0].Name = %q", cfgs[0].Name)
	}
	if cfgs[1].Name != "alibabacloud-qwen-image-2" {
		t.Errorf("cfgs[1].Name = %q, want alibabacloud-qwen-image-2", cfgs[1].Name)
	}
}

func TestParseURI_AutoNameWithModel(t *testing.T) {
	cfg, err := ParseURI("openai://sk-abc?model=dall-e-3")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "openai-dall-e-3" {
		t.Errorf("Name = %q, want openai-dall-e-3", cfg.Name)
	}
}

// --- 优化: NewFromEnv ---

func TestNewFromEnv_Set(t *testing.T) {
	os.Setenv(EnvEngineURIs, "openai://sk-test?model=dall-e-3")
	defer os.Unsetenv(EnvEngineURIs)

	cfgs, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("got %d configs, want 1", len(cfgs))
	}
	if cfgs[0].Provider != "openai" || cfgs[0].Model != "dall-e-3" {
		t.Errorf("cfg = %+v", cfgs[0])
	}
}

func TestNewFromEnv_NotSet(t *testing.T) {
	os.Unsetenv(EnvEngineURIs)
	cfgs, err := NewFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfgs != nil {
		t.Errorf("expected nil when env not set, got %+v", cfgs)
	}
}

// --- 优化: URI 脱敏 ---

func TestRedactURI(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"dashscope://sk-abc123@host.com/v1", "dashscope://sk-***@host.com/v1"},
		{"openai://sk-longapikey?model=dall-e-3", "openai://sk-***?model=dall-e-3"},
		{"kling://ak-123:sk-456@api.com/v1", "kling://ak-***:sk-***@api.com/v1"},
		{"x://ab", "x://***"},
		{"invalid-no-scheme", "invalid-no-scheme"},
		{"dashscope://sk-abc123", "dashscope://sk-***"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := RedactURI(tc.input)
			if got != tc.want {
				t.Errorf("RedactURI(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRedactURIs(t *testing.T) {
	input := "dashscope://sk-abc123?model=x,openai://sk-longkey@host.com"
	got := RedactURIs(input)
	if !strings.Contains(got, "sk-***") {
		t.Errorf("RedactURIs should mask keys, got: %s", got)
	}
	if strings.Contains(got, "sk-abc123") || strings.Contains(got, "sk-longkey") {
		t.Errorf("RedactURIs should NOT contain raw keys, got: %s", got)
	}
}

// --- 优化: ToURI 反向生成 ---

func TestToURI_Basic(t *testing.T) {
	cfg := EngineConfig{
		Provider: "openai",
		APIKey:   "sk-test",
		Model:    "dall-e-3",
		Name:     "openai-dall-e-3",
	}
	uri := ToURI(cfg)
	if !strings.HasPrefix(uri, "openai://sk-test") {
		t.Errorf("ToURI prefix wrong: %q", uri)
	}
	if !strings.Contains(uri, "model=dall-e-3") {
		t.Errorf("ToURI missing model: %q", uri)
	}
}

func TestToURI_WithBaseURL(t *testing.T) {
	cfg := EngineConfig{
		Provider: "dashscope",
		APIKey:   "sk-xxx",
		BaseURL:  "https://proxy.com/api/v1",
		Model:    "qwen-image",
		Name:     "dashscope-qwen-image",
	}
	uri := ToURI(cfg)
	if !strings.Contains(uri, "@proxy.com/api/v1") {
		t.Errorf("ToURI missing host: %q", uri)
	}
}

func TestToURI_DualKey(t *testing.T) {
	cfg := EngineConfig{
		Provider: "kling",
		APIKey:   "ak-123",
		BaseURL:  "https://api.klingai.com/v1",
		Metadata: map[string]string{"secretKey": "sk-456", "endpoint": "https://x.com"},
	}
	uri := ToURI(cfg)
	if !strings.Contains(uri, "ak-123:sk-456@") {
		t.Errorf("ToURI missing dual key: %q", uri)
	}
	if !strings.Contains(uri, "endpoint=") {
		t.Errorf("ToURI missing metadata param: %q", uri)
	}
	if strings.Contains(uri, "secretKey=") {
		t.Errorf("ToURI should not put secretKey in query: %q", uri)
	}
}

func TestToURI_Roundtrip(t *testing.T) {
	original := "openai://sk-test@proxy.com/v1?model=dall-e-3"
	cfg, err := ParseURI(original)
	if err != nil {
		t.Fatal(err)
	}
	uri := ToURI(cfg)
	cfg2, err := ParseURI(uri)
	if err != nil {
		t.Fatalf("roundtrip parse failed: %v", err)
	}
	if cfg.Provider != cfg2.Provider || cfg.APIKey != cfg2.APIKey || cfg.BaseURL != cfg2.BaseURL || cfg.Model != cfg2.Model {
		t.Errorf("roundtrip mismatch:\n  original: %+v\n  roundtrip: %+v", cfg, cfg2)
	}
}

func TestURIImageOptionsRoundTrip(t *testing.T) {
	t.Parallel()

	cfg, err := ParseURI("newapi://sk-test@example.com/v1?model=gpt-image-1-mini&quality=high&style=vivid&background=transparent&output_format=webp&moderation=low&output_compression=72")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Quality != "high" {
		t.Fatalf("quality = %q, want high", cfg.Quality)
	}
	if cfg.Style != "vivid" {
		t.Fatalf("style = %q, want vivid", cfg.Style)
	}
	if cfg.Background != "transparent" {
		t.Fatalf("background = %q, want transparent", cfg.Background)
	}
	if cfg.OutputFormat != "webp" {
		t.Fatalf("output_format = %q, want webp", cfg.OutputFormat)
	}
	if cfg.Moderation != "low" {
		t.Fatalf("moderation = %q, want low", cfg.Moderation)
	}
	if cfg.OutputCompression != 72 {
		t.Fatalf("output_compression = %d, want 72", cfg.OutputCompression)
	}
	if cfg.Metadata != nil {
		if _, ok := cfg.Metadata["quality"]; ok {
			t.Fatalf("quality should not be stored in metadata: %#v", cfg.Metadata)
		}
	}

	uri := ToURI(cfg)
	if !strings.Contains(uri, "quality=high") {
		t.Fatalf("ToURI missing quality: %q", uri)
	}
	for _, want := range []string{"style=vivid", "background=transparent", "output_format=webp", "moderation=low", "output_compression=72"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("ToURI missing %s: %q", want, uri)
		}
	}
}

// --- 优化: 解析时校验 ---

func TestParseAndValidateURI_UnknownProvider(t *testing.T) {
	_, err := ParseAndValidateURI("unknownprovider://sk-test")
	if err == nil {
		t.Error("ParseAndValidateURI should fail for unregistered provider")
	}
	if !strings.Contains(err.Error(), "no factory registered") {
		t.Errorf("error should mention factory, got: %v", err)
	}
}
