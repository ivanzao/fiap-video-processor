package video_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string { f.n++; return fmt.Sprintf("evt-%d", f.n) }

type fakeStore struct {
	mu           sync.Mutex
	objects      map[string][]byte
	uploaded     map[string]string
	failDownload error
	failUpload   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}, uploaded: map[string]string{}}
}

func (s *fakeStore) Download(_ context.Context, key, dst string) error {
	if s.failDownload != nil {
		return s.failDownload
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return video.ErrObjectNotFound
	}
	return os.WriteFile(dst, data, 0o600)
}

func (s *fakeStore) Upload(_ context.Context, src, key, contentType string) error {
	if s.failUpload != nil {
		return s.failUpload
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = data
	s.uploaded[key] = contentType
	return nil
}

type fakeExtractor struct {
	frames int
	err    error
	seen   []extractCall
}

type extractCall struct {
	frameRate int
}

func (e *fakeExtractor) Extract(_ context.Context, videoPath, outDir string, frameRate int) (int, error) {
	e.seen = append(e.seen, extractCall{frameRate})
	if e.err != nil {
		return 0, e.err
	}
	if _, err := os.Stat(videoPath); err != nil {
		return 0, err
	}
	for i := 0; i < e.frames; i++ {
		if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("frame_%04d.png", i+1)), []byte("png"), 0o600); err != nil {
			return 0, err
		}
	}
	return e.frames, nil
}

type fakePackager struct{ err error }

func (p fakePackager) Pack(_ context.Context, dir, zipPath string) error {
	if p.err != nil {
		return p.err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	return os.WriteFile(zipPath, []byte(fmt.Sprintf("zip:%d", len(entries))), 0o600)
}

type fakeWorkspace struct{ t *testing.T }

func (w fakeWorkspace) New(string) (string, func(), error) {
	dir := w.t.TempDir()
	return dir, func() {}, nil
}

type fakeRepo struct {
	executions map[string]video.Execution
	notified   map[string]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{executions: map[string]video.Execution{}, notified: map[string]bool{}}
}

func (r *fakeRepo) FindExecution(_ context.Context, requestID string) (video.Execution, error) {
	e, ok := r.executions[requestID]
	if !ok {
		return video.Execution{}, video.ErrNotFound
	}
	return e, nil
}

func (r *fakeRepo) SaveExecution(_ context.Context, e video.Execution) error {
	r.executions[e.RequestID] = e
	return nil
}

func (r *fakeRepo) WasNotified(_ context.Context, requestID string, outcome video.Outcome) (bool, error) {
	return r.notified[requestID+"/"+string(outcome)], nil
}

func (r *fakeRepo) MarkNotified(_ context.Context, requestID string, outcome video.Outcome) error {
	r.notified[requestID+"/"+string(outcome)] = true
	return nil
}

type fakePublisher struct {
	started   []video.ProcessingStarted
	completed []video.ProcessingCompleted
	failed    []video.ProcessingFailed
	err       error

	failCompleted error
}

func (p *fakePublisher) PublishStarted(_ context.Context, e video.ProcessingStarted) error {
	p.started = append(p.started, e)
	return p.err
}

func (p *fakePublisher) PublishCompleted(_ context.Context, e video.ProcessingCompleted) error {
	p.completed = append(p.completed, e)
	if p.failCompleted != nil {
		return p.failCompleted
	}
	return p.err
}

func (p *fakePublisher) PublishFailed(_ context.Context, e video.ProcessingFailed) error {
	p.failed = append(p.failed, e)
	return p.err
}

type fakeNotifier struct {
	sent []video.Notification
	err  error
}

func (n *fakeNotifier) Send(_ context.Context, msg video.Notification) error {
	if n.err != nil {
		return n.err
	}
	n.sent = append(n.sent, msg)
	return nil
}

var errTransient = errors.New("network hiccup")
