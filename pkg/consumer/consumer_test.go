package consumer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type fakeFetcher struct {
	mu       sync.Mutex
	messages []kafka.Message
	commits  [][]kafka.Message
}

func (f *fakeFetcher) ReadMessage(ctx context.Context) (kafka.Message, error) {
	for {
		f.mu.Lock()

		if len(f.messages) > 0 {
			msg := f.messages[0]
			f.messages = f.messages[1:]
			f.mu.Unlock()

			return msg, nil
		}

		f.mu.Unlock()

		select {
		case <-ctx.Done():
			return kafka.Message{}, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (f *fakeFetcher) CommitMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commits = append(f.commits, msgs)

	return nil
}

func (f *fakeFetcher) committedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0
	for _, c := range f.commits {
		n += len(c)
	}

	return n
}

type fakeForward struct {
	mu      sync.Mutex
	written []kafka.Message
	err     error
}

func (f *fakeForward) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	if f.err != nil {
		return f.err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, msgs...)

	return nil
}

type fakeDLQ struct {
	mu      sync.Mutex
	written []kafka.Message
}

func (f *fakeDLQ) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, msgs...)

	return nil
}

type stubHandler struct {
	mu        sync.Mutex
	results   []error
	calls     int
	retryable bool
}

func (s *stubHandler) Handle(context.Context, kafka.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.results[min(s.calls, len(s.results)-1)]
	s.calls++

	return err
}

func (s *stubHandler) IsRetryable(error) bool {
	return s.retryable
}

func (s *stubHandler) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

func runConsumer(ctx context.Context, c *Consumer) chan error {
	done := make(chan error, 1)

	go func() {
		done <- c.Run(ctx)
	}()

	return done
}

func TestConsumerSuccessForwardsAndCommits(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{nil}, retryable: true}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
		GroupID:      "g",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Topic: "events", Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if len(fwd.written) != 1 {
		t.Errorf("forwarded = %+v, want 1 message", fwd.written)
	}

	if len(dlq.written) != 0 {
		t.Errorf("dlq = %+v, want empty", dlq.written)
	}
}

func TestConsumerRetryThenSuccess(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{errors.New("boom"), nil}, retryable: true}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
		BaseBackoff:  time.Millisecond,
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if handler.callCount() != 2 {
		t.Errorf("calls = %d, want 2", handler.callCount())
	}

	if len(fwd.written) != 1 {
		t.Errorf("forwarded = %d, want 1", len(fwd.written))
	}
}

func TestConsumerRetryableExhaustedGoesToDLQ(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{errors.New("boom")}, retryable: true}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
		MaxAttempts:  3,
		BaseBackoff:  time.Millisecond,
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if handler.callCount() != 3 {
		t.Errorf("calls = %d, want 3", handler.callCount())
	}

	if len(dlq.written) != 1 {
		t.Fatalf("dlq = %d, want 1", len(dlq.written))
	}

	if len(dlq.written[0].Headers) == 0 {
		t.Error("dlq message has no headers")
	}

	if len(fwd.written) != 0 {
		t.Error("exhausted message was forwarded")
	}
}

func TestConsumerNonRetryableGoesToDLQImmediately(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{errors.New("poison")}, retryable: false}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if handler.callCount() != 1 {
		t.Errorf("calls = %d, want 1", handler.callCount())
	}

	if len(dlq.written) != 1 {
		t.Errorf("dlq = %d, want 1", len(dlq.written))
	}
}

func TestConsumerDropNotForwardedNotDLQ(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{ErrDrop}, retryable: true}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if len(fwd.written) != 0 {
		t.Error("dropped message was forwarded")
	}

	if len(dlq.written) != 0 {
		t.Error("dropped message went to dlq")
	}
}

func TestConsumerForwardFailureGoesToDLQ(t *testing.T) {
	fetcher := &fakeFetcher{}
	fwd := &fakeForward{err: errors.New("broker down")}
	dlq := &fakeDLQ{}
	handler := &stubHandler{results: []error{nil}, retryable: true}

	c := New(fetcher, handler, Options{
		Topic:        "events",
		Forward:      fwd,
		ForwardTopic: "validated",
		DLQ:          dlq,
		DLQTopic:     "dlq",
		MaxAttempts:  2,
		BaseBackoff:  time.Millisecond,
	}, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := runConsumer(ctx, c)

	fetcher.mu.Lock()
	fetcher.messages = append(fetcher.messages, kafka.Message{Value: []byte("v1")})
	fetcher.mu.Unlock()

	waitFor(t, 3*time.Second, func() bool { return fetcher.committedCount() == 1 }, "commit")

	cancel()
	<-done

	if len(dlq.written) != 1 {
		t.Errorf("dlq = %d, want 1", len(dlq.written))
	}
}

func TestConsumerNilReader(t *testing.T) {
	c := New(nil, &stubHandler{}, Options{}, zap.NewNop())

	if err := c.Run(context.Background()); err == nil {
		t.Error("Run with nil reader = nil, want error")
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, what string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", what)
}
