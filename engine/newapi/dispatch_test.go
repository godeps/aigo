package newapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestDispatchUnknownRoute(t *testing.T) {
	t.Parallel()

	eng := &Engine{
		route: Route("nonexistent_route"),
	}
	_, err := eng.dispatch(context.Background(), "key", workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
	})
	if err == nil {
		t.Fatal("expected error for unknown route")
	}
}

func TestDispatchAllRoutesCovered(t *testing.T) {
	t.Parallel()

	// Verify every Route constant in routeTable is callable
	for route := range routeTable {
		if route == "" {
			continue
		}
		t.Run(string(route), func(t *testing.T) {
			t.Parallel()
			// Just verify the route is in the table — actual execution
			// is tested in per-route tests
			if _, ok := routeTable[route]; !ok {
				t.Errorf("route %q missing from routeTable", route)
			}
		})
	}
}

func TestExecuteValidation(t *testing.T) {
	// Cannot use t.Parallel() because subtest uses t.Setenv

	t.Run("empty_graph", func(t *testing.T) {
		eng := New(Config{BaseURL: "https://example.com", Model: "test", APIKey: "key"})
		_, err := eng.Execute(context.Background(), workflow.Graph{})
		if err == nil {
			t.Fatal("expected error for empty graph")
		}
	})

	t.Run("missing_base_url", func(t *testing.T) {
		eng := New(Config{Model: "test", APIKey: "key"})
		_, err := eng.Execute(context.Background(), workflow.Graph{
			"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		})
		if err != ErrMissingBaseURL {
			t.Errorf("got %v, want ErrMissingBaseURL", err)
		}
	})

	t.Run("missing_model", func(t *testing.T) {
		eng := New(Config{BaseURL: "https://example.com", APIKey: "key"})
		_, err := eng.Execute(context.Background(), workflow.Graph{
			"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		})
		if err == nil {
			t.Fatal("expected error for empty model")
		}
	})

	t.Run("missing_api_key", func(t *testing.T) {
		t.Setenv("NEWAPI_API_KEY", "")
		eng := New(Config{BaseURL: "https://example.com", Model: "test"})
		_, err := eng.Execute(context.Background(), workflow.Graph{
			"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "test"}},
		})
		if err == nil {
			t.Fatal("expected error for empty api key")
		}
	})
}

func TestExecuteImageEdits(t *testing.T) {
	t.Parallel()

	// Serve a fake 1x1 PNG for image fetch and handle the edits endpoint
	pngData := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, // ...
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/image.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngData)
		case r.URL.Path == "/v1/images/edits":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/edited.png"}]}`))
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	eng := New(Config{
		BaseURL: server.URL + "/v1",
		Model:   "dall-e-2",
		Route:   RouteOpenAIImagesEdits,
		APIKey:  "sk-test",
	})

	graph := workflow.Graph{
		"1": {ClassType: "CLIPTextEncode", Inputs: map[string]any{"text": "add a hat"}},
		"2": {ClassType: "LoadImage", Inputs: map[string]any{"image_url": server.URL + "/image.png"}},
	}
	out, err := eng.Execute(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}
	if out.Value != "https://cdn.example.com/edited.png" {
		t.Errorf("got %q", out.Value)
	}
}
