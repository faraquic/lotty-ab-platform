package main

import (
	"testing"

	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"go.uber.org/zap"
)

func TestNewSnapshotConsumerWithoutBrokers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Runtime.Kafka.GroupID = "labp-runtime-snapshot"

	// kafka.NewReader panics on an empty broker list, and Kafka is optional:
	// compose and the documented quick start leave KAFKA_BROKERS empty. The
	// reader must degrade to redis-only instead of taking the process down.
	k := newSnapshotConsumer(cfg, zap.NewNop())
	if k != nil {
		t.Fatal("expected nil snapshot consumer without brokers")
	}
}

func TestNewSnapshotConsumerWithBrokers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Runtime.Kafka.GroupID = "labp-runtime-snapshot"
	cfg.Database.Kafka.Brokers = "localhost:9092, localhost:9093"

	k := newSnapshotConsumer(cfg, zap.NewNop())
	if k == nil {
		t.Fatal("expected a snapshot consumer when brokers are configured")
	}

	defer k.Close()
}
