package kafkax

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// Config controls Kafka reader and writer construction.
type Config struct {
	Brokers      []string           `mapstructure:"brokers" yaml:"brokers"`
	Topic        string             `mapstructure:"topic" yaml:"topic"`
	GroupID      string             `mapstructure:"group_id" yaml:"group_id"`
	ClientID     string             `mapstructure:"client_id" yaml:"client_id"`
	RequiredAcks kafka.RequiredAcks `mapstructure:"required_acks" yaml:"required_acks"`
	DialTimeout  time.Duration      `mapstructure:"dial_timeout" yaml:"dial_timeout"`
	Producer     ProducerConfig     `mapstructure:"producer" yaml:"producer"`
	Consumer     ConsumerConfig     `mapstructure:"consumer" yaml:"consumer"`
	SASL         SASLConfig         `mapstructure:"sasl" yaml:"sasl"`
	TLS          TLSConfig          `mapstructure:"tls" yaml:"tls"`
	// BatchTimeout is kept for backward compatibility. Prefer producer.batch_timeout.
	BatchTimeout time.Duration `mapstructure:"batch_timeout" yaml:"batch_timeout"`
}

// ProducerConfig controls Kafka writer behavior.
type ProducerConfig struct {
	MaxAttempts            int           `mapstructure:"max_attempts" yaml:"max_attempts"`
	BatchSize              int           `mapstructure:"batch_size" yaml:"batch_size"`
	BatchBytes             int64         `mapstructure:"batch_bytes" yaml:"batch_bytes"`
	BatchTimeout           time.Duration `mapstructure:"batch_timeout" yaml:"batch_timeout"`
	ReadTimeout            time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout           time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	Async                  bool          `mapstructure:"async" yaml:"async"`
	Compression            string        `mapstructure:"compression" yaml:"compression"`
	AllowAutoTopicCreation bool          `mapstructure:"allow_auto_topic_creation" yaml:"allow_auto_topic_creation"`
}

// ConsumerConfig controls Kafka reader behavior.
type ConsumerConfig struct {
	QueueCapacity         int           `mapstructure:"queue_capacity" yaml:"queue_capacity"`
	MinBytes              int           `mapstructure:"min_bytes" yaml:"min_bytes"`
	MaxBytes              int           `mapstructure:"max_bytes" yaml:"max_bytes"`
	MaxWait               time.Duration `mapstructure:"max_wait" yaml:"max_wait"`
	ReadBatchTimeout      time.Duration `mapstructure:"read_batch_timeout" yaml:"read_batch_timeout"`
	CommitInterval        time.Duration `mapstructure:"commit_interval" yaml:"commit_interval"`
	HeartbeatInterval     time.Duration `mapstructure:"heartbeat_interval" yaml:"heartbeat_interval"`
	SessionTimeout        time.Duration `mapstructure:"session_timeout" yaml:"session_timeout"`
	RebalanceTimeout      time.Duration `mapstructure:"rebalance_timeout" yaml:"rebalance_timeout"`
	StartOffset           string        `mapstructure:"start_offset" yaml:"start_offset"`
	WatchPartitionChanges bool          `mapstructure:"watch_partition_changes" yaml:"watch_partition_changes"`
	MaxAttempts           int           `mapstructure:"max_attempts" yaml:"max_attempts"`
}

// SASLConfig controls optional Kafka SASL authentication.
type SASLConfig struct {
	Enable    bool   `mapstructure:"enable" yaml:"enable"`
	Mechanism string `mapstructure:"mechanism" yaml:"mechanism"`
	Username  string `mapstructure:"username" yaml:"username"`
	Password  string `mapstructure:"password" yaml:"password"`
}

// TLSConfig controls optional Kafka TLS transport.
type TLSConfig struct {
	Enable             bool   `mapstructure:"enable" yaml:"enable"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	ServerName         string `mapstructure:"server_name" yaml:"server_name"`
}

// DefaultConfig returns local-development Kafka defaults.
func DefaultConfig() Config {
	return Config{
		Brokers:      []string{"127.0.0.1:9092"},
		Topic:        "xiaolanshu-events",
		GroupID:      "xiaolanshu-consumer",
		ClientID:     "xiaolanshu",
		RequiredAcks: kafka.RequireAll,
		DialTimeout:  5 * time.Second,
		Producer: ProducerConfig{
			MaxAttempts:            10,
			BatchSize:              100,
			BatchBytes:             1024 * 1024,
			BatchTimeout:           10 * time.Millisecond,
			ReadTimeout:            10 * time.Second,
			WriteTimeout:           10 * time.Second,
			Compression:            "none",
			AllowAutoTopicCreation: false,
		},
		Consumer: ConsumerConfig{
			QueueCapacity:         100,
			MinBytes:              1,
			MaxBytes:              10 * 1024 * 1024,
			MaxWait:               10 * time.Second,
			ReadBatchTimeout:      10 * time.Second,
			CommitInterval:        0,
			HeartbeatInterval:     3 * time.Second,
			SessionTimeout:        30 * time.Second,
			RebalanceTimeout:      30 * time.Second,
			StartOffset:           "first",
			WatchPartitionChanges: true,
			MaxAttempts:           3,
		},
		BatchTimeout: 10 * time.Millisecond,
	}
}

// Normalize applies defaults while preserving caller-provided fields.
func (cfg Config) Normalize() (Config, error) {
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
	if cfg.ClientID == "" {
		cfg.ClientID = defaults.ClientID
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DialTimeout
	}
	if cfg.BatchTimeout != 0 && cfg.Producer.BatchTimeout == 0 {
		cfg.Producer.BatchTimeout = cfg.BatchTimeout
	}
	normalizeProducerConfig(&cfg.Producer, defaults.Producer)
	normalizeConsumerConfig(&cfg.Consumer, defaults.Consumer)
	if _, err := parseCompression(cfg.Producer.Compression); err != nil {
		return Config{}, err
	}
	if _, err := parseStartOffset(cfg.Consumer.StartOffset); err != nil {
		return Config{}, err
	}
	if cfg.SASL.Enable {
		if _, err := cfg.saslMechanism(); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func normalizeProducerConfig(cfg *ProducerConfig, defaults ProducerConfig) {
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = defaults.BatchSize
	}
	if cfg.BatchBytes == 0 {
		cfg.BatchBytes = defaults.BatchBytes
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = defaults.BatchTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaults.WriteTimeout
	}
	if cfg.Compression == "" {
		cfg.Compression = defaults.Compression
	}
}

func normalizeConsumerConfig(cfg *ConsumerConfig, defaults ConsumerConfig) {
	if cfg.QueueCapacity == 0 {
		cfg.QueueCapacity = defaults.QueueCapacity
	}
	if cfg.MinBytes == 0 {
		cfg.MinBytes = defaults.MinBytes
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = defaults.MaxBytes
	}
	if cfg.MaxWait == 0 {
		cfg.MaxWait = defaults.MaxWait
	}
	if cfg.ReadBatchTimeout == 0 {
		cfg.ReadBatchTimeout = defaults.ReadBatchTimeout
	}
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = defaults.HeartbeatInterval
	}
	if cfg.SessionTimeout == 0 {
		cfg.SessionTimeout = defaults.SessionTimeout
	}
	if cfg.RebalanceTimeout == 0 {
		cfg.RebalanceTimeout = defaults.RebalanceTimeout
	}
	if cfg.StartOffset == "" {
		cfg.StartOffset = defaults.StartOffset
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = defaults.MaxAttempts
	}
}

// NewWriter creates a Kafka writer from configured brokers and topic.
func NewWriter(cfg Config) *kafka.Writer {
	normalized, _ := cfg.Normalize()
	compression, _ := parseCompression(normalized.Producer.Compression)
	return &kafka.Writer{
		Addr:                   kafka.TCP(normalized.Brokers...),
		Topic:                  normalized.Topic,
		RequiredAcks:           normalized.RequiredAcks,
		MaxAttempts:            normalized.Producer.MaxAttempts,
		BatchSize:              normalized.Producer.BatchSize,
		BatchBytes:             normalized.Producer.BatchBytes,
		BatchTimeout:           normalized.Producer.BatchTimeout,
		ReadTimeout:            normalized.Producer.ReadTimeout,
		WriteTimeout:           normalized.Producer.WriteTimeout,
		Async:                  normalized.Producer.Async,
		Compression:            compression,
		AllowAutoTopicCreation: normalized.Producer.AllowAutoTopicCreation,
		Transport:              normalized.transport(),
	}
}

// NewReader creates a Kafka reader from configured brokers, topic and group.
func NewReader(cfg Config) *kafka.Reader {
	normalized, _ := cfg.Normalize()
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:               normalized.Brokers,
		Topic:                 normalized.Topic,
		GroupID:               normalized.GroupID,
		Dialer:                normalized.dialer(),
		QueueCapacity:         normalized.Consumer.QueueCapacity,
		MinBytes:              normalized.Consumer.MinBytes,
		MaxBytes:              normalized.Consumer.MaxBytes,
		MaxWait:               normalized.Consumer.MaxWait,
		ReadBatchTimeout:      normalized.Consumer.ReadBatchTimeout,
		CommitInterval:        normalized.Consumer.CommitInterval,
		HeartbeatInterval:     normalized.Consumer.HeartbeatInterval,
		SessionTimeout:        normalized.Consumer.SessionTimeout,
		RebalanceTimeout:      normalized.Consumer.RebalanceTimeout,
		StartOffset:           normalized.startOffset(),
		WatchPartitionChanges: normalized.Consumer.WatchPartitionChanges,
		MaxAttempts:           normalized.Consumer.MaxAttempts,
	})
}

// Message is the application-facing Kafka message shape.
type Message struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
	Time    time.Time
}

// Writer is the minimal producer dependency used by Producer.
type Writer interface {
	WriteMessages(ctx context.Context, messages ...kafka.Message) error
	Close() error
}

// Producer wraps Kafka writing with a stable application API.
type Producer struct {
	writer         Writer
	writerHasTopic bool
}

// NewProducer creates a high-level Kafka producer.
func NewProducer(cfg Config) (*Producer, error) {
	normalized, err := cfg.Normalize()
	if err != nil {
		return nil, err
	}
	return &Producer{
		writer:         NewWriter(normalized),
		writerHasTopic: strings.TrimSpace(normalized.Topic) != "",
	}, nil
}

// NewProducerWithWriter creates a producer from an existing writer.
func NewProducerWithWriter(writer Writer) *Producer {
	return &Producer{writer: writer}
}

// Publish sends one business-facing message to Kafka.
func (p *Producer) Publish(ctx context.Context, message Message) error {
	if p == nil || p.writer == nil {
		return errors.New("kafka producer writer is nil")
	}
	return p.writer.WriteMessages(ctx, toKafkaMessage(message, p.writerHasTopic))
}

// PublishRaw sends native kafka-go messages.
func (p *Producer) PublishRaw(ctx context.Context, messages ...kafka.Message) error {
	if p == nil || p.writer == nil {
		return errors.New("kafka producer writer is nil")
	}
	return p.writer.WriteMessages(ctx, messages...)
}

// Close releases producer resources.
func (p *Producer) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

// Reader is the minimal consumer dependency used by Consumer.
type Reader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, messages ...kafka.Message) error
	Close() error
}

// Handler processes one consumed Kafka message.
type Handler func(ctx context.Context, message kafka.Message) error

// Consumer wraps Kafka reading with explicit handler and commit semantics.
type Consumer struct {
	reader Reader
}

// NewConsumer creates a high-level Kafka consumer.
func NewConsumer(cfg Config) (*Consumer, error) {
	if _, err := cfg.Normalize(); err != nil {
		return nil, err
	}
	return NewConsumerWithReader(NewReader(cfg)), nil
}

// NewConsumerWithReader creates a consumer from an existing reader.
func NewConsumerWithReader(reader Reader) *Consumer {
	return &Consumer{reader: reader}
}

// Consume fetches one message, handles it, and commits only on success.
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	if c == nil || c.reader == nil {
		return errors.New("kafka consumer reader is nil")
	}
	if handler == nil {
		return errors.New("kafka consumer handler is nil")
	}
	message, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return err
	}
	if err := handler(ctx, message); err != nil {
		return err
	}
	return c.reader.CommitMessages(ctx, message)
}

// Run consumes messages until the context is canceled or an error occurs.
func (c *Consumer) Run(ctx context.Context, handler Handler) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.Consume(ctx, handler); err != nil {
			return err
		}
	}
}

// Close releases consumer resources.
func (c *Consumer) Close() error {
	if c == nil || c.reader == nil {
		return nil
	}
	return c.reader.Close()
}

func toKafkaMessage(message Message, omitTopic bool) kafka.Message {
	headers := make([]kafka.Header, 0, len(message.Headers))
	keys := make([]string, 0, len(message.Headers))
	for key := range message.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		headers = append(headers, kafka.Header{Key: key, Value: []byte(message.Headers[key])})
	}
	msg := kafka.Message{
		Topic:   message.Topic,
		Key:     []byte(message.Key),
		Value:   message.Value,
		Headers: headers,
		Time:    message.Time,
	}
	if omitTopic {
		msg.Topic = ""
	}
	return msg
}

func (cfg Config) transport() *kafka.Transport {
	transport := &kafka.Transport{
		ClientID:    cfg.ClientID,
		DialTimeout: cfg.DialTimeout,
		TLS:         cfg.tlsConfig(),
	}
	if cfg.SASL.Enable {
		mechanism, err := cfg.saslMechanism()
		if err == nil {
			transport.SASL = mechanism
		}
	}
	return transport
}

func (cfg Config) dialer() *kafka.Dialer {
	dialer := &kafka.Dialer{
		ClientID: cfg.ClientID,
		Timeout:  cfg.DialTimeout,
		TLS:      cfg.tlsConfig(),
	}
	if cfg.SASL.Enable {
		mechanism, err := cfg.saslMechanism()
		if err == nil {
			dialer.SASLMechanism = mechanism
		}
	}
	return dialer
}

func (cfg Config) tlsConfig() *tls.Config {
	if !cfg.TLS.Enable {
		return nil
	}
	return &tls.Config{
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		ServerName:         cfg.TLS.ServerName,
		MinVersion:         tls.VersionTLS12,
	}
}

func (cfg Config) saslMechanism() (sasl.Mechanism, error) {
	mechanism := strings.ToLower(strings.TrimSpace(cfg.SASL.Mechanism))
	switch mechanism {
	case "", "plain":
		return plain.Mechanism{
			Username: cfg.SASL.Username,
			Password: cfg.SASL.Password,
		}, nil
	case "scram-sha-256", "scram_sha_256":
		return scram.Mechanism(scram.SHA256, cfg.SASL.Username, cfg.SASL.Password)
	case "scram-sha-512", "scram_sha_512":
		return scram.Mechanism(scram.SHA512, cfg.SASL.Username, cfg.SASL.Password)
	default:
		return nil, fmt.Errorf("unsupported kafka sasl mechanism %q", cfg.SASL.Mechanism)
	}
}

func parseCompression(value string) (kafka.Compression, error) {
	var compression compress.Compression
	if value == "" {
		return kafka.Compression(compression), nil
	}
	if err := compression.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(value)))); err != nil {
		return 0, err
	}
	return kafka.Compression(compression), nil
}

func (cfg Config) startOffset() int64 {
	offset, err := parseStartOffset(cfg.Consumer.StartOffset)
	if err != nil {
		return kafka.FirstOffset
	}
	return offset
}

func parseStartOffset(value string) (int64, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "first", "earliest", "oldest":
		return kafka.FirstOffset, nil
	case "last", "latest", "newest":
		return kafka.LastOffset, nil
	default:
		return 0, fmt.Errorf("unsupported kafka consumer start_offset %q", value)
	}
}
