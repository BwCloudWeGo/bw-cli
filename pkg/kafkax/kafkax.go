package kafkax

import (
	"time"

	"github.com/segmentio/kafka-go"
)

// Config controls Kafka reader and writer construction.
type Config struct {
	Brokers      []string           `mapstructure:"brokers" yaml:"brokers"`
	Topic        string             `mapstructure:"topic" yaml:"topic"`
	GroupID      string             `mapstructure:"group_id" yaml:"group_id"`
	RequiredAcks kafka.RequiredAcks `mapstructure:"required_acks" yaml:"required_acks"`
	BatchTimeout time.Duration      `mapstructure:"batch_timeout" yaml:"batch_timeout"`
}

// DefaultConfig returns local-development Kafka defaults.
func DefaultConfig() Config {
	return Config{
		Brokers:      []string{"127.0.0.1:9092"},
		Topic:        "xiaolanshu-events",
		GroupID:      "xiaolanshu-consumer",
		RequiredAcks: kafka.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
	}
}

// NewWriter creates a Kafka writer from configured brokers and topic.
func NewWriter(cfg Config) *kafka.Writer {
	if len(cfg.Brokers) == 0 {
		cfg.Brokers = DefaultConfig().Brokers
	}
	if cfg.Topic == "" {
		cfg.Topic = DefaultConfig().Topic
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = DefaultConfig().BatchTimeout
	}
	return &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		RequiredAcks: cfg.RequiredAcks,
		BatchTimeout: cfg.BatchTimeout,
	}
}

// NewReader creates a Kafka reader from configured brokers, topic and group.
func NewReader(cfg Config) *kafka.Reader {
	defaults := DefaultConfig()
	if len(cfg.Brokers) == 0 {
		cfg.Brokers = defaults.Brokers
	}
	if cfg.Topic == "" {
		cfg.Topic = defaults.Topic
	}
	if cfg.GroupID == "" {
		cfg.GroupID = defaults.GroupID
	}
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers: cfg.Brokers,
		Topic:   cfg.Topic,
		GroupID: cfg.GroupID,
	})
}
