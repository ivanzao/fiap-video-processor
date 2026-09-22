package video

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Config struct {
	MaxUploadBytes int64
	UploadTTL      time.Duration
	DownloadTTL    time.Duration
}

type Service struct {
	repo    Repository
	store   ObjectStore
	clock   Clock
	ids     IDGenerator
	metrics Metrics
	cfg     Config
}

type Option func(*Service)

func WithMetrics(m Metrics) Option {
	return func(s *Service) { s.metrics = m }
}

func NewService(repo Repository, store ObjectStore, clock Clock, ids IDGenerator, cfg Config, opts ...Option) *Service {
	s := &Service{repo: repo, store: store, clock: clock, ids: ids, metrics: NopMetrics{}, cfg: cfg}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type UploadCommand struct {
	Filename    string
	ContentType string
	SizeBytes   int64
}

type UploadTicket struct {
	VideoID       string
	UploadURL     string
	UploadHeaders map[string]string
	ExpiresAt     time.Time
}

func (s *Service) CreateVideo(ctx context.Context, identity Identity, cmd UploadCommand) (UploadTicket, error) {
	ext, ok := extensionOf(cmd.Filename)
	if !ok {
		return UploadTicket{}, ErrUnsupportedFormat
	}
	if cmd.SizeBytes > s.cfg.MaxUploadBytes {
		return UploadTicket{}, ErrVideoTooLarge
	}
	v := Video{
		ID:          s.ids.NewID(),
		UserID:      identity.UserID,
		Filename:    cmd.Filename,
		ContentType: cmd.ContentType,
		SizeBytes:   cmd.SizeBytes,
		CreatedAt:   s.clock.Now(),
	}
	v.ObjectKey = fmt.Sprintf("uploads/%s/%s%s", v.UserID, v.ID, ext)
	if err := s.repo.SaveVideo(ctx, v); err != nil {
		return UploadTicket{}, err
	}
	upload, err := s.store.PresignUpload(ctx, v.ObjectKey, v.ContentType, v.SizeBytes, v.owner(), s.cfg.UploadTTL)
	if err != nil {
		return UploadTicket{}, err
	}
	s.metrics.UploadRequested()
	return UploadTicket{VideoID: v.ID, UploadURL: upload.URL, UploadHeaders: upload.Headers, ExpiresAt: v.CreatedAt.Add(s.cfg.UploadTTL)}, nil
}

func (s *Service) ProcessVideo(ctx context.Context, identity Identity, videoID string) (RequestSummary, error) {
	v, err := s.ownedVideo(ctx, identity, videoID)
	if err != nil {
		return RequestSummary{}, err
	}
	if existing, err := s.repo.FindRequestByVideo(ctx, v.ID); err == nil {
		return RequestSummary{Request: existing, Video: v}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return RequestSummary{}, err
	}
	object, err := s.store.Head(ctx, v.ObjectKey)
	if err != nil {
		return RequestSummary{}, err
	}
	if object.Owner != v.owner() {
		return RequestSummary{}, ErrUploadMismatch
	}
	now := s.clock.Now()
	req := ProcessRequest{
		ID:        s.ids.NewID(),
		VideoID:   v.ID,
		UserID:    v.UserID,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	evt := ProcessingRequested{
		RequestID: req.ID, VideoID: v.ID, UserID: v.UserID, UserEmail: identity.Email,
		ObjectKey: v.ObjectKey, Filename: v.Filename,
	}
	if err := s.repo.CreateRequest(ctx, req, evt); errors.Is(err, ErrRequestExists) {
		existing, err := s.repo.FindRequestByVideo(ctx, v.ID)
		return RequestSummary{Request: existing, Video: v}, err
	} else if err != nil {
		return RequestSummary{}, err
	}
	s.metrics.StatusChanged(req.Status)
	return RequestSummary{Request: req, Video: v}, nil
}

func (s *Service) ownedVideo(ctx context.Context, identity Identity, videoID string) (Video, error) {
	v, err := s.repo.FindVideo(ctx, videoID)
	if err != nil {
		return Video{}, err
	}
	if v.UserID != identity.UserID {
		return Video{}, ErrForbidden
	}
	return v, nil
}

func (s *Service) ListVideos(ctx context.Context, identity Identity) ([]RequestSummary, error) {
	return s.repo.ListByUser(ctx, identity.UserID)
}

func (s *Service) DownloadResult(ctx context.Context, identity Identity, videoID string) (string, error) {
	v, err := s.ownedVideo(ctx, identity, videoID)
	if err != nil {
		return "", err
	}
	req, err := s.repo.FindRequestByVideo(ctx, v.ID)
	if err != nil {
		return "", err
	}
	if req.Status != StatusCompleted {
		return "", ErrNotCompleted
	}
	res, err := s.repo.FindResult(ctx, req.ID)
	if err != nil {
		return "", err
	}
	return s.store.PresignDownload(ctx, res.ZipKey, s.cfg.DownloadTTL)
}
