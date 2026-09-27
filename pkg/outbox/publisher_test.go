package outbox

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRetryDelay(t *testing.T) {
	p := NewPublisher(nil, nil, nil, zap.NewNop(),
		WithRetryDelays(time.Second, time.Minute),
	)

	tests := []struct {
		name     string
		attempts int
		want     time.Duration
	}{
		{name: "first attempt uses base", attempts: 0, want: time.Second},
		{name: "grows exponentially", attempts: 1, want: 2 * time.Second},
		{name: "grows exponentially further", attempts: 3, want: 8 * time.Second},
		{name: "caps at max", attempts: 10, want: time.Minute},
		{name: "negative clamps to base", attempts: -1, want: time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.retryDelay(tt.attempts); got != tt.want {
				t.Errorf("retryDelay(%d) = %v, want %v", tt.attempts, got, tt.want)
			}
		})
	}
}

func TestPublisherOptions(t *testing.T) {
	p := NewPublisher(nil, nil, nil, zap.NewNop(),
		WithBatchSize(10),
		WithPollInterval(100*time.Millisecond),
		WithMaxAttempts(3),
		WithPublishTimeout(time.Second),
		WithBatchSize(0),
		WithPollInterval(0),
		WithMaxAttempts(0),
		WithPublishTimeout(0),
	)

	if p.batchSize != 10 {
		t.Errorf("batchSize = %d, want 10", p.batchSize)
	}

	if p.pollInterval != 100*time.Millisecond {
		t.Errorf("pollInterval = %v, want 100ms", p.pollInterval)
	}

	if p.maxAttempts != 3 {
		t.Errorf("maxAttempts = %d, want 3", p.maxAttempts)
	}

	if p.publishTimeout != time.Second {
		t.Errorf("publishTimeout = %v, want 1s", p.publishTimeout)
	}
}
