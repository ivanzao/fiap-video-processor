package video

import (
	"context"
	"time"
)

type Repository interface {
	SaveVideo(ctx context.Context, v Video) error
	FindVideo(ctx context.Context, id string) (Video, error)
	FindRequest(ctx context.Context, id string) (ProcessRequest, error)
	FindRequestByVideo(ctx context.Context, videoID string) (ProcessRequest, error)
	CreateRequest(ctx context.Context, req ProcessRequest, evt ProcessingRequested) error
	ListByUser(ctx context.Context, userID string) ([]RequestSummary, error)
	FindResult(ctx context.Context, requestID string) (ProcessResult, error)
	WasEventProcessed(ctx context.Context, eventID string) (bool, error)
	ApplyEvent(ctx context.Context, eventID string, req ProcessRequest, res *ProcessResult) error
}

type ObjectStore interface {
	PresignUpload(ctx context.Context, key, contentType string, sizeBytes int64, owner ObjectOwner, ttl time.Duration) (PresignedUpload, error)
	Head(ctx context.Context, key string) (ObjectInfo, error)
	PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type Metrics interface {
	UploadRequested()
	StatusChanged(status Status)
}

type NopMetrics struct{}

func (NopMetrics) UploadRequested()     {}
func (NopMetrics) StatusChanged(Status) {}
