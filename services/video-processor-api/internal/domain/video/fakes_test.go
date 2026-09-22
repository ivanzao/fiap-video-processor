package video_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/domain/video"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeIDs struct {
	mu   sync.Mutex
	next int
}

func (f *fakeIDs) NewID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	return fmt.Sprintf("id-%d", f.next)
}

type fakeStore struct {
	objects   map[string]video.ObjectOwner
	presigned []presignCall
	downloads []string
}

type presignCall struct {
	key         string
	contentType string
	sizeBytes   int64
	owner       video.ObjectOwner
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string]video.ObjectOwner{}} }

func (s *fakeStore) PresignUpload(_ context.Context, key, contentType string, sizeBytes int64, owner video.ObjectOwner, _ time.Duration) (video.PresignedUpload, error) {
	s.presigned = append(s.presigned, presignCall{key, contentType, sizeBytes, owner})
	return video.PresignedUpload{URL: "https://store.local/upload/" + key, Headers: map[string]string{"x-owner": owner.UserID + "/" + owner.VideoID}}, nil
}

func (s *fakeStore) Head(_ context.Context, key string) (video.ObjectInfo, error) {
	owner, ok := s.objects[key]
	if !ok {
		return video.ObjectInfo{}, video.ErrUploadNotFound
	}
	return video.ObjectInfo{Owner: owner}, nil
}

func (s *fakeStore) putAs(key string, owner video.ObjectOwner) { s.objects[key] = owner }

func (s *fakeStore) PresignDownload(_ context.Context, key string, _ time.Duration) (string, error) {
	s.downloads = append(s.downloads, key)
	return "https://store.local/download/" + key, nil
}

type fakeRepo struct {
	videos    map[string]video.Video
	requests  map[string]video.ProcessRequest
	results   map[string]video.ProcessResult
	outbox    []video.ProcessingRequested
	processed map[string]bool

	failNextCreateWithExists bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		videos:    map[string]video.Video{},
		requests:  map[string]video.ProcessRequest{},
		results:   map[string]video.ProcessResult{},
		processed: map[string]bool{},
	}
}

func (r *fakeRepo) SaveVideo(_ context.Context, v video.Video) error {
	r.videos[v.ID] = v
	return nil
}

func (r *fakeRepo) FindVideo(_ context.Context, id string) (video.Video, error) {
	v, ok := r.videos[id]
	if !ok {
		return video.Video{}, video.ErrNotFound
	}
	return v, nil
}

func (r *fakeRepo) FindRequestByVideo(_ context.Context, videoID string) (video.ProcessRequest, error) {
	for _, req := range r.requests {
		if req.VideoID == videoID {
			return req, nil
		}
	}
	return video.ProcessRequest{}, video.ErrNotFound
}

func (r *fakeRepo) FindRequest(_ context.Context, id string) (video.ProcessRequest, error) {
	req, ok := r.requests[id]
	if !ok {
		return video.ProcessRequest{}, video.ErrNotFound
	}
	return req, nil
}

func (r *fakeRepo) CreateRequest(_ context.Context, req video.ProcessRequest, evt video.ProcessingRequested) error {
	if r.failNextCreateWithExists {
		r.failNextCreateWithExists = false
		winner := req
		winner.ID = "winner"
		r.requests[winner.ID] = winner
		return video.ErrRequestExists
	}
	r.requests[req.ID] = req
	r.outbox = append(r.outbox, evt)
	return nil
}

func (r *fakeRepo) ListByUser(_ context.Context, userID string) ([]video.RequestSummary, error) {
	var out []video.RequestSummary
	for _, req := range r.requests {
		if req.UserID != userID {
			continue
		}
		s := video.RequestSummary{Request: req, Video: r.videos[req.VideoID]}
		if res, ok := r.results[req.ID]; ok {
			s.Result = &res
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Request.CreatedAt.After(out[j].Request.CreatedAt) })
	return out, nil
}

func (r *fakeRepo) FindResult(_ context.Context, requestID string) (video.ProcessResult, error) {
	res, ok := r.results[requestID]
	if !ok {
		return video.ProcessResult{}, video.ErrNotFound
	}
	return res, nil
}

func (r *fakeRepo) WasEventProcessed(_ context.Context, eventID string) (bool, error) {
	return r.processed[eventID], nil
}

func (r *fakeRepo) ApplyEvent(_ context.Context, eventID string, req video.ProcessRequest, res *video.ProcessResult) error {
	r.requests[req.ID] = req
	if res != nil {
		r.results[req.ID] = *res
	}
	r.processed[eventID] = true
	return nil
}

type mutableClock struct{ now time.Time }

func (c *mutableClock) Now() time.Time { return c.now }
