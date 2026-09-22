package video

import (
	"context"
	"time"
)

type ObjectStore interface {
	Download(ctx context.Context, key, dst string) error
	Upload(ctx context.Context, src, key, contentType string) error
}

type Extractor interface {
	Extract(ctx context.Context, videoPath, outDir string, frameRate int) (int, error)
}

type Packager interface {
	Pack(ctx context.Context, dir, zipPath string) error
}

type Workspace interface {
	New(requestID string) (dir string, cleanup func(), err error)
}

type Repository interface {
	FindExecution(ctx context.Context, requestID string) (Execution, error)
	SaveExecution(ctx context.Context, e Execution) error
	WasNotified(ctx context.Context, requestID string, outcome Outcome) (bool, error)
	MarkNotified(ctx context.Context, requestID string, outcome Outcome) error
}

type Publisher interface {
	PublishStarted(ctx context.Context, e ProcessingStarted) error
	PublishCompleted(ctx context.Context, e ProcessingCompleted) error
	PublishFailed(ctx context.Context, e ProcessingFailed) error
}

type Notifier interface {
	Send(ctx context.Context, n Notification) error
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type Metrics interface {
	ExecutionFinished(outcome Outcome, duration time.Duration, frames int)
}

type NopMetrics struct{}

func (NopMetrics) ExecutionFinished(Outcome, time.Duration, int) {}
