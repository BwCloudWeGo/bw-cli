package rocketmqx

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/stretchr/testify/require"
)

type fakeSyncSender struct {
	started bool
	closed  bool
	sent    []*primitive.Message
}

func (s *fakeSyncSender) Start() error {
	s.started = true
	return nil
}

func (s *fakeSyncSender) Shutdown() error {
	s.closed = true
	return nil
}

func (s *fakeSyncSender) SendSync(_ context.Context, messages ...*primitive.Message) (*primitive.SendResult, error) {
	s.sent = append(s.sent, messages...)
	return &primitive.SendResult{Status: primitive.SendOK, MsgID: "msg-1"}, nil
}

type fakeTransactionSender struct {
	started bool
	sent    []*primitive.Message
}

func (s *fakeTransactionSender) Start() error {
	s.started = true
	return nil
}

func (s *fakeTransactionSender) Shutdown() error {
	return nil
}

func (s *fakeTransactionSender) SendMessageInTransaction(_ context.Context, message *primitive.Message) (*primitive.TransactionSendResult, error) {
	s.sent = append(s.sent, message)
	return &primitive.TransactionSendResult{
		SendResult: &primitive.SendResult{Status: primitive.SendOK, MsgID: "tx-1"},
		State:      primitive.CommitMessageState,
	}, nil
}

type fakePushConsumer struct {
	started       bool
	closed        bool
	subscriptions []Subscription
	handler       ConsumeHandler
}

func (c *fakePushConsumer) Start() error {
	c.started = true
	return nil
}

func (c *fakePushConsumer) Shutdown() error {
	c.closed = true
	return nil
}

func (c *fakePushConsumer) Subscribe(topic string, selector consumer.MessageSelector, handler func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error {
	c.subscriptions = append(c.subscriptions, Subscription{Topic: topic, Tags: []string{selector.Expression}})
	c.handler = func(ctx context.Context, msg MessageExt) error {
		result, err := handler(ctx, &primitive.MessageExt{Message: *toPrimitiveMessage(msg.Message)})
		if result != consumer.ConsumeSuccess {
			return errors.New("consume retry later")
		}
		return err
	}
	return nil
}

func TestSimpleProducerPublishesPrimitiveMessage(t *testing.T) {
	sender := &fakeSyncSender{}
	producer := NewSimpleProducerWithSender(sender)

	result, err := producer.Publish(context.Background(), Message{
		Topic: "note-events",
		Tag:   "note.created",
		Key:   "note-1",
		Body:  []byte(`{"id":"note-1"}`),
		Properties: map[string]string{
			"source": "note-service",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "msg-1", result.MessageID)
	require.Equal(t, "send_ok", result.Status)
	require.Len(t, sender.sent, 1)
	require.Equal(t, "note-events", sender.sent[0].Topic)
	require.Equal(t, "note.created", sender.sent[0].GetTags())
	require.Equal(t, "note-1", sender.sent[0].GetKeys())
	require.Equal(t, "note-service", sender.sent[0].GetProperty("source"))
}

func TestDelayProducerSetsDelayLevel(t *testing.T) {
	sender := &fakeSyncSender{}
	producer := NewDelayProducerWithSender(sender)

	_, err := producer.PublishDelay(context.Background(), DelayMessage{
		Message: Message{Topic: "order-events", Key: "order-1", Body: []byte("timeout")},
		Level:   3,
	})

	require.NoError(t, err)
	require.Len(t, sender.sent, 1)
	require.Equal(t, "3", sender.sent[0].GetProperty("DELAY"))
}

func TestTransactionProducerUsesLocalCallbacks(t *testing.T) {
	sender := &fakeTransactionSender{}
	producer := NewTransactionProducerWithSender(sender, TransactionCallbacks{
		ExecuteLocal: func(context.Context, Message) (TransactionState, error) {
			return CommitTransaction, nil
		},
		CheckLocal: func(context.Context, MessageExt) TransactionState {
			return RollbackTransaction
		},
	})

	result, err := producer.PublishTransaction(context.Background(), TransactionMessage{
		Message: Message{Topic: "pay-events", Key: "pay-1", Body: []byte("pay")},
	})

	require.NoError(t, err)
	require.Equal(t, "tx-1", result.MessageID)
	require.Len(t, sender.sent, 1)
}

func TestTransactionListenerMapsCallbacks(t *testing.T) {
	executed := false
	checked := false
	listener := transactionListener{callbacks: TransactionCallbacks{
		ExecuteLocal: func(context.Context, Message) (TransactionState, error) {
			executed = true
			return CommitTransaction, nil
		},
		CheckLocal: func(context.Context, MessageExt) TransactionState {
			checked = true
			return RollbackTransaction
		},
	}}

	executeState := listener.ExecuteLocalTransaction(toPrimitiveMessage(Message{Topic: "pay-events", Body: []byte("pay")}))
	checkState := listener.CheckLocalTransaction(&primitive.MessageExt{Message: *toPrimitiveMessage(Message{Topic: "pay-events"})})

	require.Equal(t, primitive.CommitMessageState, executeState)
	require.Equal(t, primitive.RollbackMessageState, checkState)
	require.True(t, executed)
	require.True(t, checked)
}

func TestPushConsumerMapsHandlerResult(t *testing.T) {
	raw := &fakePushConsumer{}
	consumer := NewPushConsumerWithClient(raw)
	called := false

	err := consumer.Subscribe(context.Background(), Subscription{
		Topic: "note-events",
		Tags:  []string{"note.created", "note.updated"},
	}, func(_ context.Context, msg MessageExt) error {
		called = true
		require.Equal(t, "note-events", msg.Topic)
		return nil
	})

	require.NoError(t, err)
	require.True(t, raw.started)
	require.Len(t, raw.subscriptions, 1)
	require.Equal(t, "note.created || note.updated", raw.subscriptions[0].Tags[0])
	require.NoError(t, raw.handler(context.Background(), MessageExt{
		Message: Message{Topic: "note-events", Body: []byte("body"), CreatedAt: time.Now()},
	}))
	require.True(t, called)
}
