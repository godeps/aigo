package newapi

import "testing"

func TestJimengNonZeroContinuePoll(t *testing.T) {
	t.Parallel()
	if !jimengNonZeroContinuePoll(429, "too many") {
		t.Fatal("429 should continue")
	}
	if !jimengNonZeroContinuePoll(100, "task is processing") {
		t.Fatal("processing message should continue")
	}
	if !jimengNonZeroContinuePoll(1, "排队中") {
		t.Fatal("queue message should continue")
	}
	if jimengNonZeroContinuePoll(401, "unauthorized") {
		t.Fatal("auth error should not continue")
	}
}
