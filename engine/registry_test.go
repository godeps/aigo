package engine

import (
	"context"
	"testing"

	"github.com/godeps/aigo/workflow"
)

type stubEngine struct{}

func (s *stubEngine) Execute(_ context.Context, _ workflow.Graph) (Result, error) {
	return Result{Value: "stub"}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("test", Entry{Engine: &stubEngine{}})

	e, ok := r.Get("test")
	if !ok {
		t.Fatal("expected to find 'test'")
	}
	if e.Name != "test" {
		t.Errorf("expected name 'test', got %q", e.Name)
	}
}

func TestRegistry_GetMissing(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	_, ok := r.Get("nope")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistry_List(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("b", Entry{Engine: &stubEngine{}})
	r.Register("a", Entry{Engine: &stubEngine{}})
	r.Register("c", Entry{Engine: &stubEngine{}})

	names := r.List()
	if len(names) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(names))
	}
	if names[0] != "a" || names[1] != "b" || names[2] != "c" {
		t.Errorf("expected sorted, got %v", names)
	}
}

func TestRegistry_FindByCapability(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("img-engine", Entry{
		Engine: &stubEngine{},
		ModelsByCapability: func() map[string][]string {
			return map[string][]string{"image": {"model-a"}}
		},
	})
	r.Register("vid-engine", Entry{
		Engine: &stubEngine{},
		ModelsByCapability: func() map[string][]string {
			return map[string][]string{"video": {"model-b"}}
		},
	})

	imgs := r.FindByCapability("image")
	if len(imgs) != 1 {
		t.Errorf("expected 1 image engine, got %d", len(imgs))
	}

	vids := r.FindByCapability("video")
	if len(vids) != 1 {
		t.Errorf("expected 1 video engine, got %d", len(vids))
	}

	audios := r.FindByCapability("audio")
	if len(audios) != 0 {
		t.Errorf("expected 0 audio engines, got %d", len(audios))
	}
}

func TestRegistry_AllModels(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("eng1", Entry{
		Engine: &stubEngine{},
		ModelsByCapability: func() map[string][]string {
			return map[string][]string{"image": {"m1", "m2"}}
		},
	})
	r.Register("eng2", Entry{Engine: &stubEngine{}}) // no models func

	all := r.AllModels()
	if len(all) != 1 {
		t.Errorf("expected 1 entry with models, got %d", len(all))
	}
	if models, ok := all["eng1"]["image"]; !ok || len(models) != 2 {
		t.Errorf("expected 2 image models for eng1, got %v", all["eng1"])
	}
}

func TestRegistry_AllConfigSchemas(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("eng1", Entry{
		Engine: &stubEngine{},
		ConfigSchemaFunc: func() []ConfigField {
			return []ConfigField{{Key: "apiKey", Required: true}}
		},
	})

	schemas := r.AllConfigSchemas()
	if len(schemas) != 1 {
		t.Errorf("expected 1 schema, got %d", len(schemas))
	}
	if len(schemas["eng1"]) != 1 {
		t.Errorf("expected 1 field, got %d", len(schemas["eng1"]))
	}
}

func TestRegistry_Len(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	if r.Len() != 0 {
		t.Errorf("expected 0, got %d", r.Len())
	}
	r.Register("a", Entry{Engine: &stubEngine{}})
	if r.Len() != 1 {
		t.Errorf("expected 1, got %d", r.Len())
	}
}

func TestRegistry_ReplaceExisting(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register("eng", Entry{
		Engine: &stubEngine{},
		ConfigSchemaFunc: func() []ConfigField {
			return []ConfigField{{Key: "old"}}
		},
	})
	r.Register("eng", Entry{
		Engine: &stubEngine{},
		ConfigSchemaFunc: func() []ConfigField {
			return []ConfigField{{Key: "new"}}
		},
	})

	if r.Len() != 1 {
		t.Errorf("expected 1 after replace, got %d", r.Len())
	}
	schemas := r.AllConfigSchemas()
	if schemas["eng"][0].Key != "new" {
		t.Errorf("expected replaced schema, got %v", schemas["eng"])
	}
}
