package poll

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseVideoJSONV2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        []byte
		wantResult  string
		wantDone    bool
		wantErr     string // substring match; empty means no error
		wantPercent float64
	}{
		{
			name:     "status queued",
			body:     mustJSON(t, VideoTask{Status: "queued"}),
			wantDone: false,
		},
		{
			name:     "status in_progress",
			body:     mustJSON(t, VideoTask{Status: "in_progress"}),
			wantDone: false,
		},
		{
			name:     "status empty",
			body:     mustJSON(t, VideoTask{Status: ""}),
			wantDone: false,
		},
		{
			name:       "completed with url",
			body:       mustJSON(t, VideoTask{Status: "completed", URL: "https://example.com/video.mp4"}),
			wantResult: "https://example.com/video.mp4",
			wantDone:   true,
			wantPercent: 1.0,
		},
		{
			name:     "completed without url",
			body:     mustJSON(t, VideoTask{Status: "completed"}),
			wantDone: true,
			wantErr:  "no url",
		},
		{
			name:     "completed with whitespace-only url",
			body:     mustJSON(t, VideoTask{Status: "completed", URL: "   "}),
			wantDone: true,
			wantErr:  "no url",
		},
		{
			name: "failed with error message",
			body: mustJSON(t, map[string]any{
				"status": "failed",
				"error":  map[string]any{"message": "rate limit exceeded", "code": 429},
			}),
			wantDone: true,
			wantErr:  "rate limit exceeded",
		},
		{
			name: "failed without error",
			body: mustJSON(t, map[string]any{
				"status": "failed",
			}),
			wantDone: true,
			wantErr:  "failed",
		},
		{
			name: "failed with empty error message",
			body: mustJSON(t, map[string]any{
				"status": "failed",
				"error":  map[string]any{"message": "", "code": 500},
			}),
			wantDone: true,
			wantErr:  "failed",
		},
		{
			name:     "unknown status falls through to default",
			body:     mustJSON(t, VideoTask{Status: "processing"}),
			wantDone: false,
		},
		{
			name:    "invalid json",
			body:    []byte(`{not json`),
			wantErr: "decode video task",
		},
		{
			name: "in_progress with task metrics",
			body: mustJSON(t, VideoTask{
				Status:      "in_progress",
				TaskMetrics: &TaskMetrics{Total: 10, Succeeded: 3, Failed: 0},
			}),
			wantDone:    false,
			wantPercent: 0.3,
		},
		{
			name: "queued with task metrics zero total",
			body: mustJSON(t, VideoTask{
				Status:      "queued",
				TaskMetrics: &TaskMetrics{Total: 0, Succeeded: 0, Failed: 0},
			}),
			wantDone:    false,
			wantPercent: 0.0,
		},
		{
			name: "unknown status with task metrics",
			body: mustJSON(t, VideoTask{
				Status:      "pending",
				TaskMetrics: &TaskMetrics{Total: 4, Succeeded: 2, Failed: 0},
			}),
			wantDone:    false,
			wantPercent: 0.5,
		},
		{
			name: "completed with url ignores task metrics",
			body: mustJSON(t, VideoTask{
				Status:      "completed",
				URL:         "https://example.com/v.mp4",
				TaskMetrics: &TaskMetrics{Total: 5, Succeeded: 3, Failed: 0},
			}),
			wantResult:  "https://example.com/v.mp4",
			wantDone:    true,
			wantPercent: 1.0,
		},
		{
			name:       "completed with url containing whitespace is trimmed",
			body:       mustJSON(t, VideoTask{Status: "completed", URL: "  https://example.com/v.mp4  "}),
			wantResult: "https://example.com/v.mp4",
			wantDone:   true,
			wantPercent: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ParseVideoJSONV2(tt.body)

			if got.Result != tt.wantResult {
				t.Errorf("Result = %q, want %q", got.Result, tt.wantResult)
			}
			if got.Done != tt.wantDone {
				t.Errorf("Done = %v, want %v", got.Done, tt.wantDone)
			}
			if tt.wantErr != "" {
				if got.Err == nil {
					t.Fatalf("Err = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(got.Err.Error(), tt.wantErr) {
					t.Errorf("Err = %q, want substring %q", got.Err.Error(), tt.wantErr)
				}
			} else {
				if got.Err != nil {
					t.Errorf("Err = %v, want nil", got.Err)
				}
			}
			if got.Percent != tt.wantPercent {
				t.Errorf("Percent = %v, want %v", got.Percent, tt.wantPercent)
			}
		})
	}
}

func TestParseVideoJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       []byte
		wantURL    string
		wantDone   bool
		wantErr    string
	}{
		{
			name:     "delegates to V2 - queued",
			body:     mustJSON(t, VideoTask{Status: "queued"}),
			wantDone: false,
		},
		{
			name:     "delegates to V2 - completed with url",
			body:     mustJSON(t, VideoTask{Status: "completed", URL: "https://example.com/out.mp4"}),
			wantURL:  "https://example.com/out.mp4",
			wantDone: true,
		},
		{
			name:     "delegates to V2 - failed",
			body:     mustJSON(t, map[string]any{"status": "failed", "error": map[string]any{"message": "timeout"}}),
			wantDone: true,
			wantErr:  "timeout",
		},
		{
			name:    "delegates to V2 - invalid json",
			body:    []byte(`!!!`),
			wantErr: "decode video task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			url, done, err := ParseVideoJSON(tt.body)

			if url != tt.wantURL {
				t.Errorf("url = %q, want %q", url, tt.wantURL)
			}
			if done != tt.wantDone {
				t.Errorf("done = %v, want %v", done, tt.wantDone)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("err = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %q, want substring %q", err.Error(), tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}
		})
	}
}

func TestVideoParseResult_ToFetchResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       VideoParseResult
		wantResult  string
		wantDone    bool
		wantPercent float64
		wantErr     string
	}{
		{
			name:        "completed result",
			input:       VideoParseResult{Result: "https://example.com/v.mp4", Done: true, Percent: 1.0},
			wantResult:  "https://example.com/v.mp4",
			wantDone:    true,
			wantPercent: 1.0,
		},
		{
			name:        "in progress with percent",
			input:       VideoParseResult{Done: false, Percent: 0.5},
			wantDone:    false,
			wantPercent: 0.5,
		},
		{
			name:    "error result",
			input:   VideoParseResult{Done: true, Err: fmt.Errorf("some error")},
			wantDone: true,
			wantErr: "some error",
		},
		{
			name:  "zero value",
			input: VideoParseResult{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fr, err := tt.input.ToFetchResult()

			if fr.Result != tt.wantResult {
				t.Errorf("FetchResult.Result = %q, want %q", fr.Result, tt.wantResult)
			}
			if fr.Done != tt.wantDone {
				t.Errorf("FetchResult.Done = %v, want %v", fr.Done, tt.wantDone)
			}
			if fr.Percent != tt.wantPercent {
				t.Errorf("FetchResult.Percent = %v, want %v", fr.Percent, tt.wantPercent)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("err = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %q, want substring %q", err.Error(), tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			}
		})
	}
}

// mustJSON marshals v to JSON bytes, failing the test on error.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return b
}
