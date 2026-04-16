package poll

import "strings"

// TaskStatus represents the normalized status of an async task.
type TaskStatus int

const (
	StatusPending   TaskStatus = iota // queued, not yet started
	StatusRunning                     // in progress
	StatusSuccess                     // completed successfully
	StatusFailed                      // completed with error
	StatusCancelled                   // cancelled by user or system
)

// String returns a human-readable status name.
func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRunning:
		return "running"
	case StatusSuccess:
		return "success"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

// Done returns true if the status is terminal (success, failed, or cancelled).
func (s TaskStatus) Done() bool {
	return s == StatusSuccess || s == StatusFailed || s == StatusCancelled
}

// MapStatus maps a raw status string from any engine API to a normalized TaskStatus.
// The mapping is case-insensitive and covers common variations across providers.
func MapStatus(raw string) TaskStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	// Success variants
	case "success", "succeed", "succeeded", "completed", "complete", "done", "finished":
		return StatusSuccess

	// Failed variants
	case "failed", "fail", "failure", "error", "errored":
		return StatusFailed

	// Cancelled variants
	case "cancelled", "canceled", "cancel", "aborted", "timeout", "timed_out":
		return StatusCancelled

	// Pending variants
	case "pending", "queued", "queue", "waiting", "not-started", "not_started",
		"created", "create", "submitted", "preparing", "scheduled":
		return StatusPending

	// Running variants
	case "running", "processing", "in_progress", "in-progress", "active",
		"generating", "uploading", "started", "queueing":
		return StatusRunning

	default:
		return StatusRunning // treat unknown as still in progress
	}
}
