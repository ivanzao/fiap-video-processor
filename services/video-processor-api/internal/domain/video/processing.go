package video

import (
	"context"
	"fmt"
)

func (s *Service) StartProcessing(ctx context.Context, eventID, requestID string, attempt int) error {
	return s.apply(ctx, eventID, requestID, func(req *ProcessRequest) (*ProcessResult, error) {
		if req.Status.terminal() {
			return nil, nil
		}
		if err := req.transition(StatusProcessing); err != nil {
			return nil, err
		}
		req.Attempts = max(req.Attempts, attempt)
		return nil, nil
	})
}

func (s *Service) CompleteProcessing(ctx context.Context, eventID, requestID, zipKey string, frameCount int) error {
	return s.apply(ctx, eventID, requestID, func(req *ProcessRequest) (*ProcessResult, error) {
		if err := req.transition(StatusCompleted); err != nil {
			return nil, err
		}
		return &ProcessResult{RequestID: req.ID, ZipKey: zipKey, FrameCount: frameCount, RecordedAt: s.clock.Now()}, nil
	})
}

func (s *Service) FailProcessing(ctx context.Context, eventID, requestID, reason string, retryable bool, attempt int) error {
	return s.apply(ctx, eventID, requestID, func(req *ProcessRequest) (*ProcessResult, error) {
		req.Attempts = max(req.Attempts, attempt)
		if retryable {
			return nil, req.transition(StatusProcessing)
		}
		if err := req.transition(StatusFailed); err != nil {
			return nil, err
		}
		return &ProcessResult{RequestID: req.ID, FailureReason: reason, RecordedAt: s.clock.Now()}, nil
	})
}

func (s *Service) apply(ctx context.Context, eventID, requestID string, change func(*ProcessRequest) (*ProcessResult, error)) error {
	done, err := s.repo.WasEventProcessed(ctx, eventID)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	req, err := s.repo.FindRequest(ctx, requestID)
	if err != nil {
		return err
	}
	before := req.Status
	res, err := change(&req)
	if err != nil {
		return err
	}
	req.UpdatedAt = s.clock.Now()
	if err := s.repo.ApplyEvent(ctx, eventID, req, res); err != nil {
		return err
	}
	if req.Status != before {
		s.metrics.StatusChanged(req.Status)
	}
	return nil
}

var allowedTransitions = map[Status][]Status{
	StatusPending:    {StatusProcessing, StatusCompleted, StatusFailed},
	StatusProcessing: {StatusProcessing, StatusCompleted, StatusFailed},
}

func (s Status) terminal() bool {
	return s == StatusCompleted || s == StatusFailed
}

func (r *ProcessRequest) transition(to Status) error {
	for _, allowed := range allowedTransitions[r.Status] {
		if allowed == to {
			r.Status = to
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, to)
}
