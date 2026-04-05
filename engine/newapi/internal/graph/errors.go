package graph

import "errors"

var (
	ErrMissingPrompt      = errors.New("graph: prompt not found in workflow graph")
	ErrMissingImageSource = errors.New("graph: image source for edits not found (image_url, image_b64, image_path, LoadImage)")
	ErrMissingAudioSource = errors.New("graph: audio source for whisper not found (audio_url, audio_path, audio_b64)")
)
