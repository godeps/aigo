package graph

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/godeps/aigo/workflow"
)

func TestBaseName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "unix path",
			path: "/home/user/images/photo.png",
			want: "photo.png",
		},
		{
			name: "windows backslash path",
			path: `C:\Users\test\image.jpg`,
			want: "image.jpg",
		},
		{
			name: "URL-like path",
			path: "https://example.com/files/audio.mp3",
			want: "audio.mp3",
		},
		{
			name: "simple name no separator",
			path: "file.txt",
			want: "file.txt",
		},
		{
			name: "trailing slash",
			path: "/var/log/",
			want: "",
		},
		{
			name: "nested deep path",
			path: "a/b/c/d/e.wav",
			want: "e.wav",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := baseName(tt.path)
			if got != tt.want {
				t.Fatalf("baseName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestImageBytesForEdits_Base64(t *testing.T) {
	t.Parallel()
	raw := []byte("fake-png-data")
	encoded := base64.StdEncoding.EncodeToString(raw)
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"image_b64": encoded,
		}},
	}
	got, err := ImageBytesForEdits(g, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

func TestImageBytesForEdits_Base64Alt(t *testing.T) {
	t.Parallel()
	raw := []byte("another-image")
	encoded := base64.StdEncoding.EncodeToString(raw)
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"image_base64": encoded,
		}},
	}
	got, err := ImageBytesForEdits(g, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

func TestImageBytesForEdits_MissingSource(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Other", Inputs: map[string]any{"prompt": "hello"}},
	}
	_, err := ImageBytesForEdits(g, nil, false)
	if !errors.Is(err, ErrMissingImageSource) {
		t.Fatalf("got error %v, want ErrMissingImageSource", err)
	}
}

func TestImageBytesForEdits_EmptyGraph(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{}
	_, err := ImageBytesForEdits(g, nil, false)
	if !errors.Is(err, ErrMissingImageSource) {
		t.Fatalf("got error %v, want ErrMissingImageSource", err)
	}
}

func TestImageBytesForEdits_RemoteDisabled(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"image_url": "https://example.com/image.png",
		}},
	}
	_, err := ImageBytesForEdits(g, nil, false)
	if !errors.Is(err, ErrRemoteMediaDisabled) {
		t.Fatalf("got error %v, want ErrRemoteMediaDisabled", err)
	}
}

func TestImageBytesForEdits_RemoteDisabledEditURL(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"edit_image_url": "https://example.com/edit.png",
		}},
	}
	_, err := ImageBytesForEdits(g, nil, false)
	if !errors.Is(err, ErrRemoteMediaDisabled) {
		t.Fatalf("got error %v, want ErrRemoteMediaDisabled", err)
	}
}

func TestAudioBytesForWhisper_Base64(t *testing.T) {
	t.Parallel()
	raw := []byte("fake-audio-data")
	encoded := base64.StdEncoding.EncodeToString(raw)
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_b64": encoded,
		}},
	}
	fn, got, err := AudioBytesForWhisper(g, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fn != "audio.bin" {
		t.Fatalf("filename = %q, want %q", fn, "audio.bin")
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

func TestAudioBytesForWhisper_Base64WithFilename(t *testing.T) {
	t.Parallel()
	raw := []byte("fake-audio")
	encoded := base64.StdEncoding.EncodeToString(raw)
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_b64":      encoded,
			"audio_filename": "speech.wav",
		}},
	}
	fn, got, err := AudioBytesForWhisper(g, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fn != "speech.wav" {
		t.Fatalf("filename = %q, want %q", fn, "speech.wav")
	}
	if string(got) != string(raw) {
		t.Fatalf("got %q, want %q", got, raw)
	}
}

func TestAudioBytesForWhisper_MissingSource(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Other", Inputs: map[string]any{"prompt": "hello"}},
	}
	_, _, err := AudioBytesForWhisper(g, nil, false)
	if !errors.Is(err, ErrMissingAudioSource) {
		t.Fatalf("got error %v, want ErrMissingAudioSource", err)
	}
}

func TestAudioBytesForWhisper_RemoteDisabled(t *testing.T) {
	t.Parallel()
	g := workflow.Graph{
		"1": {ClassType: "Options", Inputs: map[string]any{
			"audio_url": "https://example.com/audio.mp3",
		}},
	}
	_, _, err := AudioBytesForWhisper(g, nil, false)
	if !errors.Is(err, ErrRemoteMediaDisabled) {
		t.Fatalf("got error %v, want ErrRemoteMediaDisabled", err)
	}
}
