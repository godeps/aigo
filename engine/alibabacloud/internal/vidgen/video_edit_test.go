package vidgen

import (
	"errors"
	"testing"

	"github.com/godeps/aigo/engine/alibabacloud/internal/ierr"
)

func TestValidateWanVideoEditMedia(t *testing.T) {
	tests := []struct {
		name  string
		media []map[string]any
		want  error
	}{
		{"valid_video_only", []map[string]any{{"type": "video", "url": "v.mp4"}}, nil},
		{"valid_video_plus_2_images", []map[string]any{
			{"type": "video", "url": "v.mp4"},
			{"type": "reference_image", "url": "1.png"},
			{"type": "reference_image", "url": "2.png"},
		}, nil},
		{"valid_video_plus_4_images", []map[string]any{
			{"type": "video", "url": "v.mp4"},
			{"type": "reference_image", "url": "1.png"},
			{"type": "reference_image", "url": "2.png"},
			{"type": "reference_image", "url": "3.png"},
			{"type": "reference_image", "url": "4.png"},
		}, nil},
		{"no_media", nil, ierr.ErrWanVideoEditMissingVideo},
		{"no_video", []map[string]any{{"type": "reference_image", "url": "1.png"}}, ierr.ErrWanVideoEditMissingVideo},
		{"two_videos", []map[string]any{
			{"type": "video", "url": "a.mp4"},
			{"type": "video", "url": "b.mp4"},
		}, ierr.ErrWanVideoEditMissingVideo},
		{"five_images", []map[string]any{
			{"type": "video", "url": "v.mp4"},
			{"type": "reference_image", "url": "1.png"},
			{"type": "reference_image", "url": "2.png"},
			{"type": "reference_image", "url": "3.png"},
			{"type": "reference_image", "url": "4.png"},
			{"type": "reference_image", "url": "5.png"},
		}, ierr.ErrWanVideoEditTooManyImages},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWanVideoEditMedia(tt.media)
			if !errors.Is(err, tt.want) {
				t.Fatalf("validateWanVideoEditMedia() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRunVideoEdit_MissingVideo(t *testing.T) {
	g := promptGraph("edit this")
	_, err := RunVideoEdit(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrWanVideoEditMissingVideo) {
		t.Fatalf("expected ErrWanVideoEditMissingVideo, got %v", err)
	}
}

func TestRunVideoEdit_MissingPrompt(t *testing.T) {
	g := imageGraph("https://example.com/a.png")
	_, err := RunVideoEdit(nil, nil, "", "", g)
	if !errors.Is(err, ierr.ErrMissingPrompt) {
		t.Fatalf("expected ErrMissingPrompt, got %v", err)
	}
}
