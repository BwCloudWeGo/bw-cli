package kafkax

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

type captureWriter struct {
	messages []kafka.Message
}

func (w *captureWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *captureWriter) Close() error {
	return nil
}

func TestProducerPublishOmitsMessageTopicWhenWriterUsesConfiguredTopic(t *testing.T) {
	writer := &captureWriter{}
	producer := &Producer{writer: writer, writerHasTopic: true}

	err := producer.Publish(context.Background(), Message{
		Topic: "message-topic",
		Key:   "order-created",
		Value: []byte(`{"order_id":"1"}`),
	})

	require.NoError(t, err)
	require.Len(t, writer.messages, 1)
	require.Empty(t, writer.messages[0].Topic)
	require.Equal(t, []byte("order-created"), writer.messages[0].Key)
	require.Equal(t, []byte(`{"order_id":"1"}`), writer.messages[0].Value)
}

func TestProducerPublishKeepsMessageTopicForInjectedRawWriter(t *testing.T) {
	writer := &captureWriter{}
	producer := NewProducerWithWriter(writer)

	err := producer.Publish(context.Background(), Message{
		Topic: "message-topic",
		Key:   "order-created",
		Value: []byte(`{"order_id":"1"}`),
	})

	require.NoError(t, err)
	require.Len(t, writer.messages, 1)
	require.Equal(t, "message-topic", writer.messages[0].Topic)
}

func TestProducerPublishDoesNotPassTopicToConfiguredKafkaWriter(t *testing.T) {
	writer := &kafka.Writer{
		Addr:  kafka.TCP("127.0.0.1:1"),
		Topic: "configured-topic",
	}
	producer := &Producer{writer: writer, writerHasTopic: true}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := producer.Publish(ctx, Message{
		Topic: "message-topic",
		Value: []byte("payload"),
	})

	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), "topic must not be specified")
}
