package poll

import "testing"

func FuzzMapStatus(f *testing.F) {
	// Seed corpus with known status strings
	for _, s := range []string{
		"Success", "Failed", "Running", "Pending", "cancelled",
		"QUEUED", "processing", "timeout", "error", "",
		"in_progress", "completed", "not-started",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// MapStatus should never panic
		status := MapStatus(input)
		// Result should be a valid TaskStatus
		if status < StatusPending || status > StatusCancelled {
			t.Errorf("MapStatus(%q) returned invalid status %d", input, status)
		}
	})
}
