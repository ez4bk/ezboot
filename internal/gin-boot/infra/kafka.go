package infra

import (
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/segmentio/kafka-go"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

// Kafka is a typed handle to the configured broker set.
type Kafka struct {
	brokers []string
}

// Brokers returns the configured Kafka broker addresses.
func (k *Kafka) Brokers() []string {
	if k == nil {
		return nil
	}
	return append([]string(nil), k.brokers...)
}

type kafkaClients struct {
	mu sync.RWMutex

	client *Kafka
}

func (k *kafkaClients) open(cfg *config.Config) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !cfg.App.Middleware.Kafka.KafkaClusterStartUp {
		return nil
	}

	brokers := cfg.App.Middleware.Kafka.KafkaClusterURL
	if env := os.Getenv("EZ_KAFKA_BROKER_URL"); env != "" {
		brokers = strings.Split(env, ",")
	}
	k.client = &Kafka{brokers: brokers}

	return nil
}

func (k *kafkaClients) get() (*Kafka, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if k.client == nil {
		return nil, resourceError("kafka", defaultName, nil)
	}
	return k.client, nil
}

func (k *kafkaClients) close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.client = nil
	return nil
}

// Kafka returns the configured Kafka broker set.
func (s *Set) Kafka() (*Kafka, error) {
	if s == nil {
		return nil, resourceError("kafka", defaultName, nil)
	}
	return s.kafka.get()
}

// NewKafkaReader creates a reader on the process-level default Kafka brokers.
//
// Deprecated: obtain *Set and use KafkaReader instead, so the reader
// uses the same explicit dependency graph as the rest of the application.
func NewKafkaReader(topic string, group string) *kafka.Reader {
	reader, err := Default().KafkaReader(topic, group)
	if err != nil {
		panic(err)
	}
	return reader
}

// KafkaReader creates a reader on this Set's Kafka brokers.
func (s *Set) KafkaReader(topic string, group string) (*kafka.Reader, error) {
	k, err := s.Kafka()
	if err != nil {
		return nil, err
	}
	if len(k.brokers) == 0 {
		return nil, resourceError("kafka", defaultName, errors.New("no brokers configured"))
	}
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.brokers,
		Topic:    topic,
		GroupID:  group,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	}), nil
}

// NewKafkaWriter creates a writer on the process-level default Kafka brokers.
//
// Deprecated: obtain *Set and use KafkaWriter instead.
func NewKafkaWriter(topic string) *kafka.Writer {
	writer, err := Default().KafkaWriter(topic)
	if err != nil {
		panic(err)
	}
	return writer
}

// KafkaWriter creates a writer on this Set's Kafka brokers.
func (s *Set) KafkaWriter(topic string) (*kafka.Writer, error) {
	k, err := s.Kafka()
	if err != nil {
		return nil, err
	}
	if len(k.brokers) == 0 {
		return nil, resourceError("kafka", defaultName, errors.New("no brokers configured"))
	}
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  k.brokers,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}), nil
}
