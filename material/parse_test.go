package material

import "testing"

func TestParseURI_Pexels(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("pexels://abc123")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "pexels" {
		t.Fatalf("scheme = %q, want pexels", p.Scheme)
	}
	if p.APIKey != "abc123" {
		t.Fatalf("APIKey = %q, want abc123", p.APIKey)
	}
}

func TestParseURI_Unsplash(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("unsplash://mykey")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "unsplash" || p.APIKey != "mykey" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseURI_Pixabay(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("pixabay://12345-key")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "pixabay" || p.APIKey != "12345-key" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseURI_OSS(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("oss://LTAI5t:wJalrXUtn@my-bucket.cn-hangzhou?mode=basic&token=sts123")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "oss" {
		t.Fatalf("scheme = %q", p.Scheme)
	}
	if p.APIKey != "LTAI5t" {
		t.Fatalf("APIKey = %q", p.APIKey)
	}
	if p.Secret != "wJalrXUtn" {
		t.Fatalf("Secret = %q", p.Secret)
	}
	if p.Bucket != "my-bucket" {
		t.Fatalf("Bucket = %q", p.Bucket)
	}
	if p.Region != "cn-hangzhou" {
		t.Fatalf("Region = %q", p.Region)
	}
	if p.Mode != "basic" {
		t.Fatalf("Mode = %q", p.Mode)
	}
	if p.SecurityToken != "sts123" {
		t.Fatalf("SecurityToken = %q", p.SecurityToken)
	}
}

func TestParseURI_OSSDefaultMode(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("oss://AK:SK@bucket.cn-beijing")
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != "semantic" {
		t.Fatalf("default mode = %q, want semantic", p.Mode)
	}
}

func TestParseURI_Local(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("local:///data/index.json?embed=jina&embed_key=KEY1&embed_model=clip-v2")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "local" {
		t.Fatalf("scheme = %q", p.Scheme)
	}
	if p.IndexPath != "/data/index.json" {
		t.Fatalf("IndexPath = %q", p.IndexPath)
	}
	if p.EmbedBackend != "jina" {
		t.Fatalf("EmbedBackend = %q", p.EmbedBackend)
	}
	if p.EmbedKey != "KEY1" {
		t.Fatalf("EmbedKey = %q", p.EmbedKey)
	}
	if p.EmbedModel != "clip-v2" {
		t.Fatalf("EmbedModel = %q", p.EmbedModel)
	}
}

func TestParseURI_LocalDefaults(t *testing.T) {
	t.Parallel()
	p, err := ParseURI("local://")
	if err != nil {
		t.Fatal(err)
	}
	if p.IndexPath != ".aigo/search_index.json" {
		t.Fatalf("IndexPath = %q", p.IndexPath)
	}
	if p.EmbedBackend != "dashscope" {
		t.Fatalf("EmbedBackend = %q", p.EmbedBackend)
	}
}

func TestParseURI_Errors(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"noscheme",
		"unknown://key",
		"pexels://",
		"oss://keyonly",
		"oss://ak:sk@bucketonly",
	}
	for _, c := range cases {
		_, err := ParseURI(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestParseURIs_Multi(t *testing.T) {
	t.Parallel()
	uris, err := ParseURIs("pexels://K1, unsplash://K2, pixabay://K3")
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != 3 {
		t.Fatalf("got %d URIs, want 3", len(uris))
	}
	if uris[0].Scheme != "pexels" || uris[1].Scheme != "unsplash" || uris[2].Scheme != "pixabay" {
		t.Fatalf("schemes = %v, %v, %v", uris[0].Scheme, uris[1].Scheme, uris[2].Scheme)
	}
}

func TestPaginationRoundTrip(t *testing.T) {
	t.Parallel()
	state := PaginationState{
		"pexels":   "2",
		"unsplash": "3",
		"oss":      "MTIzNDU2",
	}
	encoded := EncodePagination(state)
	if encoded == "" {
		t.Fatal("encoded is empty")
	}
	decoded := DecodePagination(encoded)
	if len(decoded) != 3 {
		t.Fatalf("decoded has %d entries, want 3", len(decoded))
	}
	if decoded["pexels"] != "2" || decoded["oss"] != "MTIzNDU2" {
		t.Fatalf("decoded mismatch: %v", decoded)
	}
}

func TestDecodePagination_Invalid(t *testing.T) {
	t.Parallel()
	if s := DecodePagination(""); s != nil {
		t.Fatal("empty should return nil")
	}
	if s := DecodePagination("not-valid-base64!!!"); s != nil {
		t.Fatal("invalid should return nil")
	}
}
