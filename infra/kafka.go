package infra

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/segmentio/kafka-go"

	"github.com/ez4bk/ezboot/internal/gin-boot/config"
)

// Kafka is a typed handle to the configured broker set.
type KafkaBrokers struct {
	brokers []string
}

// Brokers returns the configured Kafka broker addresses.
func (k *KafkaBrokers) Brokers() []string {
	if k == nil {
		return nil
	}
	return append([]string(nil), k.brokers...)
}

type kafkaClients struct {
	mu sync.RWMutex

	client *KafkaBrokers
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
	k.client = &KafkaBrokers{brokers: brokers}

	return nil
}

func (k *kafkaClients) get() (*KafkaBrokers, error) {
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

func (k *kafkaClients) healthy(ctx context.Context) []error {
	k.mu.RLock()
	client := k.client
	k.mu.RUnlock()

	if client == nil {
		return nil
	}
	if len(client.brokers) == 0 {
		return []error{errors.New("kafka: no brokers configured")}
	}

	var errs []error
	for _, broker := range client.brokers {
		conn, err := kafka.DialContext(ctx, "tcp", broker)
		if err != nil {
			errs = append(errs, fmt.Errorf("kafka broker %q: %w", broker, err))
			continue
		}
		_, err = conn.Controller()
		_ = conn.Close()
		if err != nil {
			errs = append(errs, fmt.Errorf("kafka broker %q: %w", broker, err))
		}
	}
	return errs
}

// Kafka returns the configured Kafka broker set.
func (s *Kafka) KafkaBrokers() (*KafkaBrokers, error) {
	if s == nil {
		return nil, resourceError("kafka", defaultName, nil)
	}
	return s.kafkaClients.get()
}

// KafkaReader creates a reader on this plugin's Kafka brokers.
func (s *Kafka) KafkaReader(topic string, group string) (*kafka.Reader, error) {
	k, err := s.KafkaBrokers()
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

// KafkaWriter creates a writer on this plugin's Kafka brokers.
func (s *Kafka) KafkaWriter(topic string) (*kafka.Writer, error) {
	k, err := s.KafkaBrokers()
	if err != nil {
		return nil, err
	}
	if len(k.brokers) == 0 {
		return nil, resourceError("kafka", defaultName, errors.New("no brokers configured"))
	}
	return &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}, nil
}
