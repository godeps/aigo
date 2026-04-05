package newapi

import "errors"

var (
	ErrMissingAPIKey  = errors.New("newapi: missing API key")
	ErrMissingPrompt  = errors.New("newapi: prompt not found in workflow graph")
	ErrMissingVoice   = errors.New("newapi: voice not found for speech (use AudioOptions.voice)")
	ErrMissingBaseURL = errors.New("newapi: BaseURL is empty (set Config.BaseURL or NEWAPI_BASE_URL)")

	ErrMissingImageSource    = errors.New("newapi: image source for edits not found in graph")
	ErrMissingAudioSource    = errors.New("newapi: audio source for whisper not found in graph")
	ErrMissingJimengReqKey   = errors.New("newapi: jimeng req_key missing (graph input req_key / jimeng_req_key)")
	ErrRemoteMediaDisabled   = errors.New("newapi: remote media URL fetch disabled (set DisableRemoteMediaFetch=false to allow image_url/audio_url)")
)
