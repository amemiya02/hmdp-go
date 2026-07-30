package global

import (
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaWriterWaitsForReplicaAcknowledgement(t *testing.T) {
	initKafkaWriter()
	t.Cleanup(func() { _ = closeKafkaWriter() })

	if KafkaWriter.Async {
		t.Fatal("Kafka writer must not ignore asynchronous delivery errors")
	}
	if KafkaWriter.RequiredAcks != kafka.RequireAll {
		t.Fatalf("required acks = %v, want RequireAll", KafkaWriter.RequiredAcks)
	}
	if _, ok := KafkaWriter.Balancer.(*kafka.Hash); !ok {
		t.Fatalf("balancer = %T, want *kafka.Hash", KafkaWriter.Balancer)
	}
	if KafkaWriter.WriteTimeout != 5*time.Second {
		t.Fatalf("write timeout = %v, want 5s", KafkaWriter.WriteTimeout)
	}
	if KafkaWriter.MaxAttempts != 1 {
		t.Fatalf("max attempts = %d, want one bounded attempt before pending relay", KafkaWriter.MaxAttempts)
	}
}
