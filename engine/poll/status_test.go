package poll

import "testing"

func TestMapStatus_Success(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"Success", "success", "SUCCESS", "Succeed", "completed", "done", "finished"} {
		if got := MapStatus(s); got != StatusSuccess {
			t.Errorf("MapStatus(%q) = %v, want StatusSuccess", s, got)
		}
	}
}

func TestMapStatus_Failed(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"Failed", "failed", "FAILED", "fail", "error", "errored"} {
		if got := MapStatus(s); got != StatusFailed {
			t.Errorf("MapStatus(%q) = %v, want StatusFailed", s, got)
		}
	}
}

func TestMapStatus_Cancelled(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"cancelled", "canceled", "cancel", "aborted", "timeout"} {
		if got := MapStatus(s); got != StatusCancelled {
			t.Errorf("MapStatus(%q) = %v, want StatusCancelled", s, got)
		}
	}
}

func TestMapStatus_Pending(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"pending", "queued", "waiting", "not-started", "create", "submitted"} {
		if got := MapStatus(s); got != StatusPending {
			t.Errorf("MapStatus(%q) = %v, want StatusPending", s, got)
		}
	}
}

func TestMapStatus_Running(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"running", "processing", "in_progress", "generating", "uploading", "started"} {
		if got := MapStatus(s); got != StatusRunning {
			t.Errorf("MapStatus(%q) = %v, want StatusRunning", s, got)
		}
	}
}

func TestMapStatus_UnknownDefaultsToRunning(t *testing.T) {
	t.Parallel()
	if got := MapStatus("some_weird_status"); got != StatusRunning {
		t.Errorf("MapStatus(unknown) = %v, want StatusRunning", got)
	}
}

func TestMapStatus_Trimmed(t *testing.T) {
	t.Parallel()
	if got := MapStatus("  Success  "); got != StatusSuccess {
		t.Errorf("MapStatus with spaces = %v, want StatusSuccess", got)
	}
}

func TestTaskStatus_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    TaskStatus
		want string
	}{
		{StatusPending, "pending"},
		{StatusRunning, "running"},
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestTaskStatus_Done(t *testing.T) {
	t.Parallel()
	if StatusRunning.Done() {
		t.Error("StatusRunning should not be done")
	}
	if StatusPending.Done() {
		t.Error("StatusPending should not be done")
	}
	if !StatusSuccess.Done() {
		t.Error("StatusSuccess should be done")
	}
	if !StatusFailed.Done() {
		t.Error("StatusFailed should be done")
	}
	if !StatusCancelled.Done() {
		t.Error("StatusCancelled should be done")
	}
}
