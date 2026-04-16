package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadReader(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("expected multipart content type, got %q", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("expected auth header, got %q", auth)
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}

		// Check extra field.
		if got := r.FormValue("description"); got != "test file" {
			t.Errorf("expected description field, got %q", got)
		}

		// Check file field.
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		if header.Filename != "photo.jpg" {
			t.Errorf("expected filename photo.jpg, got %q", header.Filename)
		}

		w.WriteHeader(200)
		w.Write([]byte(`{"id":"upload-123"}`))
	}))
	defer srv.Close()

	body, err := UploadReader(
		context.Background(),
		http.DefaultClient,
		srv.URL,
		"test-key",
		"image",
		"photo.jpg",
		strings.NewReader("fake image data"),
		map[string]string{"description": "test file"},
		"test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "upload-123") {
		t.Errorf("unexpected response: %s", body)
	}
}

func TestUploadFile(t *testing.T) {
	t.Parallel()

	// Create a temp file.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		if header.Filename != "test.txt" {
			t.Errorf("expected filename test.txt, got %q", header.Filename)
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	body, err := UploadFile(
		context.Background(),
		http.DefaultClient,
		srv.URL,
		"",
		"file",
		path,
		nil,
		"test",
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("unexpected response: %s", body)
	}
}

func TestUploadReader_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	_, err := UploadReader(
		context.Background(),
		http.DefaultClient,
		srv.URL,
		"",
		"file",
		"test.txt",
		strings.NewReader("data"),
		nil,
		"test",
	)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
