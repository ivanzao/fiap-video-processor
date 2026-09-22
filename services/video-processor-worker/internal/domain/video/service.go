package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Dependencies struct {
	Store     ObjectStore
	Extractor Extractor
	Packager  Packager
	Workspace Workspace
	Repo      Repository
	Publisher Publisher
	Notifier  Notifier
	Clock     Clock
	IDs       IDGenerator
	Metrics   Metrics
}

type Config struct {
	FrameRate   int
	MaxAttempts int
}

type Service struct {
	deps Dependencies
	cfg  Config
}

func NewService(deps Dependencies, cfg Config) *Service {
	if deps.Metrics == nil {
		deps.Metrics = NopMetrics{}
	}
	return &Service{deps: deps, cfg: cfg}
}

func (s *Service) ExecuteProcessRequest(ctx context.Context, req ProcessRequest) error {
	previous, err := s.deps.Repo.FindExecution(ctx, req.RequestID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return retryLater(err)
	}
	if err == nil && previous.terminal() {
		return s.republish(ctx, req, previous)
	}

	exec := Execution{RequestID: req.RequestID, Attempt: req.Attempt, Outcome: OutcomeRunning, StartedAt: s.deps.Clock.Now()}
	if err := s.deps.Repo.SaveExecution(ctx, exec); err != nil {
		return retryLater(err)
	}
	if err := s.deps.Publisher.PublishStarted(ctx, ProcessingStarted{RequestID: req.RequestID, Attempt: req.Attempt}); err != nil {
		return retryLater(err)
	}

	zipKey, frames, runErr := s.run(ctx, req)
	if runErr != nil {
		return s.fail(ctx, req, exec, runErr)
	}
	exec.Outcome, exec.ZipKey, exec.FrameCount, exec.FinishedAt = OutcomeCompleted, zipKey, frames, s.deps.Clock.Now()
	if err := s.deps.Repo.SaveExecution(ctx, exec); err != nil {
		return retryLater(err)
	}
	s.deps.Metrics.ExecutionFinished(exec.Outcome, exec.FinishedAt.Sub(exec.StartedAt), frames)
	return s.announceCompleted(ctx, req, exec)
}

func (s *Service) fail(ctx context.Context, req ProcessRequest, exec Execution, cause error) error {
	definitive := isDefinitive(cause) || req.Attempt >= s.cfg.MaxAttempts
	exec.Reason, exec.FinishedAt = reasonOf(cause), s.deps.Clock.Now()
	if definitive {
		exec.Outcome = OutcomeFailed
	} else {
		exec.Outcome = OutcomeRetrying
	}
	if err := s.deps.Repo.SaveExecution(ctx, exec); err != nil {
		return retryLater(err)
	}
	s.deps.Metrics.ExecutionFinished(exec.Outcome, exec.FinishedAt.Sub(exec.StartedAt), 0)
	if err := s.deps.Publisher.PublishFailed(ctx, ProcessingFailed{RequestID: req.RequestID, Reason: exec.Reason, Retryable: !definitive, Attempt: req.Attempt}); err != nil {
		return retryLater(err)
	}
	if !definitive {
		return fmt.Errorf("%w: %w", ErrRetryLater, cause)
	}
	return s.notify(ctx, req, Notification{To: req.UserEmail, RequestID: req.RequestID, Filename: req.Filename, Outcome: OutcomeFailed, Reason: exec.Reason})
}

func (s *Service) republish(ctx context.Context, req ProcessRequest, exec Execution) error {
	if exec.Outcome == OutcomeCompleted {
		return s.announceCompleted(ctx, req, exec)
	}
	if err := s.deps.Publisher.PublishFailed(ctx, ProcessingFailed{RequestID: req.RequestID, Reason: exec.Reason, Retryable: false, Attempt: exec.Attempt}); err != nil {
		return retryLater(err)
	}
	return s.notify(ctx, req, Notification{To: req.UserEmail, RequestID: req.RequestID, Filename: req.Filename, Outcome: OutcomeFailed, Reason: exec.Reason})
}

func (s *Service) announceCompleted(ctx context.Context, req ProcessRequest, exec Execution) error {
	if err := s.deps.Publisher.PublishCompleted(ctx, ProcessingCompleted{
		RequestID: req.RequestID, ZipKey: exec.ZipKey, FrameCount: exec.FrameCount, Duration: exec.FinishedAt.Sub(exec.StartedAt),
	}); err != nil {
		return retryLater(err)
	}
	return s.notify(ctx, req, Notification{To: req.UserEmail, RequestID: req.RequestID, Filename: req.Filename, Outcome: OutcomeCompleted, FrameCount: exec.FrameCount})
}

func (s *Service) run(ctx context.Context, req ProcessRequest) (string, int, error) {
	dir, cleanup, err := s.deps.Workspace.New(req.RequestID)
	if err != nil {
		return "", 0, err
	}
	defer cleanup()
	videoPath := filepath.Join(dir, "source"+filepath.Ext(req.Filename))
	if err := s.deps.Store.Download(ctx, req.ObjectKey, videoPath); err != nil {
		return "", 0, err
	}
	framesDir := filepath.Join(dir, "frames")
	if err := os.MkdirAll(framesDir, 0o750); err != nil {
		return "", 0, err
	}
	frames, err := s.deps.Extractor.Extract(ctx, videoPath, framesDir, s.cfg.FrameRate)
	if err != nil {
		return "", 0, err
	}
	zipPath := filepath.Join(dir, "frames.zip")
	if err := s.deps.Packager.Pack(ctx, framesDir, zipPath); err != nil {
		return "", 0, err
	}
	zipKey := "results/" + req.RequestID + ".zip"
	if err := s.deps.Store.Upload(ctx, zipPath, zipKey, "application/zip"); err != nil {
		return "", 0, err
	}
	return zipKey, frames, nil
}

func (s *Service) notify(ctx context.Context, req ProcessRequest, n Notification) error {
	done, err := s.deps.Repo.WasNotified(ctx, req.RequestID, n.Outcome)
	if err != nil {
		return retryLater(err)
	}
	if done {
		return nil
	}
	if err := s.deps.Notifier.Send(ctx, n); err != nil {
		return retryLater(err)
	}
	if err := s.deps.Repo.MarkNotified(ctx, req.RequestID, n.Outcome); err != nil {
		return retryLater(err)
	}
	return nil
}

func (e Execution) terminal() bool {
	return e.Outcome == OutcomeCompleted || e.Outcome == OutcomeFailed
}

func isDefinitive(err error) bool {
	var unprocessable *UnprocessableError
	return errors.As(err, &unprocessable) || errors.Is(err, ErrObjectNotFound)
}

func reasonOf(err error) string {
	var unprocessable *UnprocessableError
	if errors.As(err, &unprocessable) {
		return "unprocessable video: " + unprocessable.Reason
	}
	if errors.Is(err, ErrObjectNotFound) {
		return "video object not found in storage"
	}
	return err.Error()
}

func retryLater(err error) error {
	return fmt.Errorf("%w: %w", ErrRetryLater, err)
}
