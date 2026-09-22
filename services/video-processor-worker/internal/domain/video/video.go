package video

import (
	"errors"
	"time"
)

var (
	ErrNotFound       = errors.New("video: not found")
	ErrObjectNotFound = errors.New("video: video object not found in storage")
	ErrRetryLater     = errors.New("video: retryable failure, message must be redelivered")
)

type UnprocessableError struct {
	Reason string
}

func (e *UnprocessableError) Error() string { return "video: unprocessable video: " + e.Reason }

type ProcessRequest struct {
	RequestID string
	VideoID   string
	UserID    string
	UserEmail string
	ObjectKey string
	Filename  string
	Attempt   int
}

type Outcome string

const (
	OutcomeRunning   Outcome = "RUNNING"
	OutcomeRetrying  Outcome = "RETRYING"
	OutcomeCompleted Outcome = "COMPLETED"
	OutcomeFailed    Outcome = "FAILED"
)

type Execution struct {
	RequestID  string
	Attempt    int
	Outcome    Outcome
	ZipKey     string
	FrameCount int
	Reason     string
	StartedAt  time.Time
	FinishedAt time.Time
}

type ProcessingStarted struct {
	RequestID string
	Attempt   int
}

type ProcessingCompleted struct {
	RequestID  string
	ZipKey     string
	FrameCount int
	Duration   time.Duration
}

type ProcessingFailed struct {
	RequestID string
	Reason    string
	Retryable bool
	Attempt   int
}

type Notification struct {
	To         string
	RequestID  string
	Filename   string
	Outcome    Outcome
	FrameCount int
	Reason     string
}
