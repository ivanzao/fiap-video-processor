package outbox_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/adapters/outbound/outbox"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-api/internal/platform/events"
)

type source struct {
	mu        sync.Mutex
	pending   []events.PendingEvent
	pendErr   error
	markCalls int
	published []int64
}

func (s *source) PendingEvents(_ context.Context, limit int) ([]events.PendingEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendErr != nil {
		return nil, s.pendErr
	}
	return s.pending[:min(limit, len(s.pending))], nil
}

func (s *source) MarkPublished(_ context.Context, ids []int64, _ time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.markCalls++
	s.published = append(s.published, ids...)
	done := map[int64]bool{}
	for _, id := range ids {
		done[id] = true
	}
	var left []events.PendingEvent
	for _, p := range s.pending {
		if !done[p.ID] {
			left = append(left, p)
		}
	}
	s.pending = left
	return nil
}

func (s *source) marked() (calls int, ids []int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.markCalls, append([]int64(nil), s.published...)
}

type publisher struct {
	failOn string
	sent   []string
}

func (p *publisher) Publish(_ context.Context, env events.Envelope) error {
	if env.EventID == p.failOn {
		return errors.New("sns unavailable")
	}
	p.sent = append(p.sent, env.EventID)
	return nil
}

func pendingEvent(id int64, eventID string) events.PendingEvent {
	return events.PendingEvent{ID: id, Envelope: events.Wrap(eventID, time.Now(), events.VideoProcessingRequested{RequestID: "req-" + eventID})}
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestDrainPublishesPendingEventsInOrderUpToTheBatch(t *testing.T) {
	src := &source{pending: []events.PendingEvent{pendingEvent(1, "e1"), pendingEvent(2, "e2"), pendingEvent(3, "e3")}}
	pub := &publisher{}

	require.NoError(t, outbox.NewRelay(src, pub, discard(), time.Second, 2).Drain(t.Context()))

	assert.Equal(t, []string{"e1", "e2"}, pub.sent)
	_, ids := src.marked()
	assert.Equal(t, []int64{1, 2}, ids)
}

func TestDrainStopsAtTheFirstFailureAndMarksOnlyWhatWasSent(t *testing.T) {
	src := &source{pending: []events.PendingEvent{pendingEvent(1, "e1"), pendingEvent(2, "e2"), pendingEvent(3, "e3")}}
	pub := &publisher{failOn: "e2"}

	require.NoError(t, outbox.NewRelay(src, pub, discard(), time.Second, 10).Drain(t.Context()))

	assert.Equal(t, []string{"e1"}, pub.sent)
	_, ids := src.marked()
	assert.Equal(t, []int64{1}, ids)
}

func TestDrainReportsSourceFailures(t *testing.T) {
	src := &source{pendErr: errors.New("db down")}

	err := outbox.NewRelay(src, &publisher{}, discard(), time.Second, 10).Drain(t.Context())

	assert.EqualError(t, err, "db down")
	calls, _ := src.marked()
	assert.Zero(t, calls)
}

func TestRunDrainsOnEveryTickUntilCancelled(t *testing.T) {
	src := &source{pendErr: errors.New("db down")}
	relay := outbox.NewRelay(src, &publisher{}, discard(), 5*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		relay.Run(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	src.mu.Lock()
	src.pendErr = nil
	src.pending = []events.PendingEvent{pendingEvent(1, "e1")}
	src.mu.Unlock()
	assert.Eventually(t, func() bool { _, ids := src.marked(); return len(ids) == 1 }, time.Second, 5*time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("relay did not stop after cancellation")
	}
}
