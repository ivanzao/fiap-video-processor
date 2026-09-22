package video

import "time"

type ProcessResult struct {
	RequestID     string
	ZipKey        string
	FrameCount    int
	FailureReason string
	RecordedAt    time.Time
}

type RequestSummary struct {
	Request ProcessRequest
	Video   Video
	Result  *ProcessResult
}
