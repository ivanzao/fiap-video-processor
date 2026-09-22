package video

import "time"

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusProcessing Status = "PROCESSING"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
)

type ProcessRequest struct {
	ID        string
	VideoID   string
	UserID    string
	Status    Status
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProcessingRequested struct {
	RequestID string
	VideoID   string
	UserID    string
	UserEmail string
	ObjectKey string
	Filename  string
}
