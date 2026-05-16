package kafkax_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/kafkax"
)

func TestDefaultConfig(t *testing.T) {
	cfg := kafkax.DefaultConfig()

	require.Equal(t, []string{"127.0.0.1:9092"}, cfg.Brokers)
	require.Equal(t, "xiaolanshu-events", cfg.Topic)
	require.Equal(t, "xiaolanshu-consumer", cfg.GroupID)
	require.Equal(t, kafka.RequireAll, cfg.RequiredAcks)
	require.Equal(t, 10, cfg.Producer.MaxAttempts)
	require.Equal(t, 100, cfg.Producer.BatchSize)
	require.Equal(t, 1, cfg.Consumer.MinBytes)
	require.Equal(t, 10*1024*1024, cfg.Consumer.MaxBytes)
	require.Equal(t, "first", cfg.Consumer.StartOffset)
	require.Equal(t, 5*time.Second, cfg.DialTimeout)
	require.False(t, cfg.TLS.Enable)
}

func TestNormalizeAppliesFieldDefaults(t *testing.T) {
	cfg := kafkax.Config{
		Brokers: []string{"kafka:9092"},
		Topic:   "orders",
	}

	normalized, err := cfg.Normalize()

	require.NoError(t, err)
	require.Equal(t, []string{"kafka:9092"}, normalized.Brokers)
	require.Equal(t, "orders", normalized.Topic)
	require.Equal(t, "xiaolanshu-consumer", normalized.GroupID)
	require.Equal(t, kafka.RequireNone, normalized.RequiredAcks)
	require.Equal(t, 100, normalized.Producer.BatchSize)
	require.Equal(t, "first", normalized.Consumer.StartOffset)
}

func TestNewWriterPreservesRequireNone(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.RequiredAcks = kafka.RequireNone

	writer := kafkax.NewWriter(cfg)

	require.Equal(t, kafka.RequireNone, writer.RequiredAcks)
}

func TestNewWriterUsesProducerConfig(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.Brokers = []string{"kafka-1:9092", "kafka-2:9092"}
	cfg.Topic = "payments"
	cfg.Producer.MaxAttempts = 5
	cfg.Producer.BatchSize = 50
	cfg.Producer.BatchBytes = 2 * 1024 * 1024
	cfg.Producer.BatchTimeout = 200 * time.Millisecond
	cfg.Producer.ReadTimeout = 3 * time.Second
	cfg.Producer.WriteTimeout = 4 * time.Second
	cfg.Producer.Async = true
	cfg.Producer.AllowAutoTopicCreation = true
	cfg.Producer.Compression = "gzip"

	writer := kafkax.NewWriter(cfg)

	require.Equal(t, "payments", writer.Topic)
	require.Equal(t, kafka.RequireAll, writer.RequiredAcks)
	require.Equal(t, 5, writer.MaxAttempts)
	require.Equal(t, 50, writer.BatchSize)
	require.Equal(t, int64(2*1024*1024), writer.BatchBytes)
	require.Equal(t, 200*time.Millisecond, writer.BatchTimeout)
	require.Equal(t, 3*time.Second, writer.ReadTimeout)
	require.Equal(t, 4*time.Second, writer.WriteTimeout)
	require.True(t, writer.Async)
	require.True(t, writer.AllowAutoTopicCreation)
	require.Equal(t, kafka.Gzip, writer.Compression)
	require.NotNil(t, writer.Transport)
}

func TestNewReaderUsesConsumerConfig(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.Brokers = []string{"kafka:9092"}
	cfg.Topic = "orders"
	cfg.GroupID = "orders-worker"
	cfg.Consumer.QueueCapacity = 256
	cfg.Consumer.MinBytes = 10
	cfg.Consumer.MaxBytes = 20
	cfg.Consumer.MaxWait = 2 * time.Second
	cfg.Consumer.CommitInterval = time.Second
	cfg.Consumer.HeartbeatInterval = 4 * time.Second
	cfg.Consumer.SessionTimeout = 40 * time.Second
	cfg.Consumer.RebalanceTimeout = 45 * time.Second
	cfg.Consumer.StartOffset = "last"
	cfg.Consumer.WatchPartitionChanges = true
	cfg.Consumer.MaxAttempts = 7

	reader := kafkax.NewReader(cfg)

	readerCfg := reader.Config()
	require.Equal(t, []string{"kafka:9092"}, readerCfg.Brokers)
	require.Equal(t, "orders", readerCfg.Topic)
	require.Equal(t, "orders-worker", readerCfg.GroupID)
	require.Equal(t, 256, readerCfg.QueueCapacity)
	require.Equal(t, 10, readerCfg.MinBytes)
	require.Equal(t, 20, readerCfg.MaxBytes)
	require.Equal(t, 2*time.Second, readerCfg.MaxWait)
	require.Equal(t, time.Second, readerCfg.CommitInterval)
	require.Equal(t, 4*time.Second, readerCfg.HeartbeatInterval)
	require.Equal(t, 40*time.Second, readerCfg.SessionTimeout)
	require.Equal(t, 45*time.Second, readerCfg.RebalanceTimeout)
	require.Equal(t, kafka.LastOffset, readerCfg.StartOffset)
	require.True(t, readerCfg.WatchPartitionChanges)
	require.Equal(t, 7, readerCfg.MaxAttempts)
	require.NotNil(t, readerCfg.Dialer)
	require.NoError(t, reader.Close())
}

func TestNormalizeRejectsUnsupportedSASLMechanism(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.SASL.Enable = true
	cfg.SASL.Mechanism = "kerberos"

	_, err := cfg.Normalize()

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported kafka sasl mechanism")
}

func TestNormalizeRejectsUnsupportedCompression(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.Producer.Compression = "brotli"

	_, err := cfg.Normalize()

	require.Error(t, err)
	require.Contains(t, err.Error(), "compression format")
}

func TestNormalizeRejectsUnsupportedStartOffset(t *testing.T) {
	cfg := kafkax.DefaultConfig()
	cfg.Consumer.StartOffset = "middle"

	_, err := cfg.Normalize()

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported kafka consumer start_offset")
}

func TestProducerPublishMapsBusinessMessage(t *testing.T) {
	writer := &fakeWriter{}
	producer := kafkax.NewProducerWithWriter(writer)
	ctx := context.Background()

	err := producer.Publish(ctx, kafkax.Message{
		Topic: "audit",
		Key:   "user-created",
		Value: []byte(`{"id":1}`),
		Headers: map[string]string{
			"trace_id": "trace-1",
		},
		Time: time.Unix(10, 0),
	})

	require.NoError(t, err)
	require.Equal(t, ctx, writer.ctx)
	require.Len(t, writer.messages, 1)
	require.Equal(t, "audit", writer.messages[0].Topic)
	require.Equal(t, []byte("user-created"), writer.messages[0].Key)
	require.Equal(t, []byte(`{"id":1}`), writer.messages[0].Value)
	require.Equal(t, time.Unix(10, 0), writer.messages[0].Time)
	require.Equal(t, []kafka.Header{{Key: "trace_id", Value: []byte("trace-1")}}, writer.messages[0].Headers)
	require.NoError(t, producer.Close())
	require.True(t, writer.closed)
}

func TestConsumerConsumeCommitsAfterHandlerSuccess(t *testing.T) {
	reader := &fakeReader{
		message: kafka.Message{Topic: "audit", Key: []byte("k"), Value: []byte("v")},
	}
	consumer := kafkax.NewConsumerWithReader(reader)

	err := consumer.Consume(context.Background(), func(ctx context.Context, msg kafka.Message) error {
		require.Equal(t, []byte("v"), msg.Value)
		return nil
	})

	require.NoError(t, err)
	require.True(t, reader.committed)
	require.Equal(t, reader.message, reader.committedMessage)
}

func TestConsumerConsumeSkipsCommitWhenHandlerFails(t *testing.T) {
	handlerErr := errors.New("handle failed")
	reader := &fakeReader{
		message: kafka.Message{Topic: "audit", Key: []byte("k"), Value: []byte("v")},
	}
	consumer := kafkax.NewConsumerWithReader(reader)

	err := consumer.Consume(context.Background(), func(context.Context, kafka.Message) error {
		return handlerErr
	})

	require.ErrorIs(t, err, handlerErr)
	require.False(t, reader.committed)
}

type fakeWriter struct {
	ctx      context.Context
	messages []kafka.Message
	closed   bool
}

func (w *fakeWriter) WriteMessages(ctx context.Context, messages ...kafka.Message) error {
	w.ctx = ctx
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *fakeWriter) Close() error {
	w.closed = true
	return nil
}

type fakeReader struct {
	message          kafka.Message
	committed        bool
	committedMessage kafka.Message
}

func (r *fakeReader) FetchMessage(context.Context) (kafka.Message, error) {
	return r.message, nil
}

func (r *fakeReader) CommitMessages(_ context.Context, messages ...kafka.Message) error {
	r.committed = true
	if len(messages) > 0 {
		r.committedMessage = messages[0]
	}
	return nil
}

func (r *fakeReader) Close() error {
	return nil
}
