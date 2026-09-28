package decide

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type fakeKafkaWriter struct {
	mu   sync.Mutex
	msgs []kafka.Message
	err  error
}

func (f *fakeKafkaWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msgs...)
	return nil
}

func (f *fakeKafkaWriter) Close() error { return nil }

func (f *fakeKafkaWriter) getMessages() []kafka.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]kafka.Message, len(f.msgs))
	copy(out, f.msgs)
	return out
}

func TestDecisionPublisher_PublishesRecord(t *testing.T) {
	fake := &fakeKafkaWriter{}
	pub := NewDecisionPublisher(fake, 10, zap.NewNop())
	defer pub.Stop()

	rec := DecisionRecord{
		DecisionID:     "11111111-1111-1111-1111-111111111111",
		RequestID:      "22222222-2222-2222-2222-222222222222",
		SubjectID:      "user-1",
		FlagKey:        "flag-a",
		ResultSource:   "experiment",
		ConfigRevision: "42",
		CreatedAt:      time.Now(),
	}

	pub.Publish(rec)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(fake.getMessages()) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	msgs := fake.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}

	expectedKey, _ := uuid.Parse(rec.DecisionID)
	if string(msgs[0].Key) != string(expectedKey[:]) {
		t.Errorf("key = %x, want %x", msgs[0].Key, expectedKey[:])
	}
}

func TestDecisionPublisher_ChannelFull_DropsRecord(t *testing.T) {
	fake := &fakeKafkaWriter{}
	pub := NewDecisionPublisher(fake, 1, zap.NewNop())
	defer pub.Stop()

	pub.Publish(DecisionRecord{DecisionID: "11111111-1111-1111-1111-111111111111"})
	pub.Publish(DecisionRecord{DecisionID: "22222222-2222-2222-2222-222222222222"})

	time.Sleep(50 * time.Millisecond)

	if len(fake.getMessages()) > 2 {
		t.Fatalf("got %d messages, want <= 2", len(fake.getMessages()))
	}
}

func TestDecisionPublisher_NilWriter_NoPanic(t *testing.T) {
	pub := NewDecisionPublisher(nil, 10, zap.NewNop())
	defer pub.Stop()

	rec := DecisionRecord{
		DecisionID:   "11111111-1111-1111-1111-111111111111",
		RequestID:    "22222222-2222-2222-2222-222222222222",
		SubjectID:    "user-1",
		FlagKey:      "flag-a",
		ResultSource: "default",
		CreatedAt:    time.Now(),
	}

	pub.Publish(rec)
}

func TestDecisionPublisher_WriteError_NotFatal(t *testing.T) {
	fake := &fakeKafkaWriter{err: context.DeadlineExceeded}
	pub := NewDecisionPublisher(fake, 10, zap.NewNop())
	defer pub.Stop()

	rec := DecisionRecord{
		DecisionID:   "11111111-1111-1111-1111-111111111111",
		RequestID:    "22222222-2222-2222-2222-222222222222",
		SubjectID:    "user-1",
		FlagKey:      "flag-a",
		ResultSource: "default",
		CreatedAt:    time.Now(),
	}

	pub.Publish(rec)

	time.Sleep(50 * time.Millisecond)
}
