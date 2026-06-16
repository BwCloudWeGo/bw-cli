package rocketmqx

import (
	"context"
	"errors"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// TransactionState 表示本地事务执行或回查后的 RocketMQ 事务状态。
type TransactionState int

const (
	// CommitTransaction 提交半消息。
	CommitTransaction TransactionState = iota + 1
	// RollbackTransaction 回滚半消息。
	RollbackTransaction
	// UnknownTransaction 让 RocketMQ 后续继续回查。
	UnknownTransaction
)

// TransactionCallbacks 封装本地事务执行和回查逻辑。
type TransactionCallbacks struct {
	ExecuteLocal func(context.Context, Message) (TransactionState, error)
	CheckLocal   func(context.Context, MessageExt) TransactionState
}

// TransactionSender 是事务生产者依赖的最小 SDK 接口。
type TransactionSender interface {
	Start() error
	Shutdown() error
	SendMessageInTransaction(context.Context, *primitive.Message) (*primitive.TransactionSendResult, error)
}

// TransactionProducer 发布 RocketMQ 事务消息。
type TransactionProducer struct {
	sender    TransactionSender
	callbacks TransactionCallbacks
}

// NewTransactionProducer 创建并启动事务消息生产者。
func NewTransactionProducer(cfg Config, callbacks TransactionCallbacks) (*TransactionProducer, error) {
	listener := transactionListener{callbacks: callbacks}
	sender, err := producer.NewTransactionProducer(listener, cfg.producerOptions()...)
	if err != nil {
		return nil, err
	}
	p := NewTransactionProducerWithSender(sender, callbacks)
	if err := p.Start(); err != nil {
		return nil, err
	}
	return p, nil
}

// NewTransactionProducerWithSender 基于已有发送器创建事务消息生产者。
func NewTransactionProducerWithSender(sender TransactionSender, callbacks TransactionCallbacks) *TransactionProducer {
	return &TransactionProducer{sender: sender, callbacks: callbacks}
}

// Start 启动底层 RocketMQ 事务生产者。
func (p *TransactionProducer) Start() error {
	if p == nil || p.sender == nil {
		return errors.New("rocketmq transaction producer sender is nil")
	}
	return p.sender.Start()
}

// PublishTransaction 同步发送一条事务半消息。
func (p *TransactionProducer) PublishTransaction(ctx context.Context, message TransactionMessage) (SendResult, error) {
	if p == nil || p.sender == nil {
		return SendResult{}, errors.New("rocketmq transaction producer sender is nil")
	}
	result, err := p.sender.SendMessageInTransaction(ctx, toPrimitiveMessage(message.Message))
	if err != nil {
		return SendResult{}, err
	}
	return toSendResult(result.SendResult), nil
}

// Close 关闭底层 RocketMQ 事务生产者。
func (p *TransactionProducer) Close() error {
	if p == nil || p.sender == nil {
		return nil
	}
	return p.sender.Shutdown()
}

type transactionListener struct {
	callbacks TransactionCallbacks
}

func (l transactionListener) ExecuteLocalTransaction(message *primitive.Message) primitive.LocalTransactionState {
	if l.callbacks.ExecuteLocal == nil {
		return primitive.UnknowState
	}
	state, err := l.callbacks.ExecuteLocal(context.Background(), fromPrimitiveMessage(message))
	if err != nil {
		return primitive.RollbackMessageState
	}
	return toPrimitiveTransactionState(state)
}

func (l transactionListener) CheckLocalTransaction(message *primitive.MessageExt) primitive.LocalTransactionState {
	if l.callbacks.CheckLocal == nil {
		return primitive.UnknowState
	}
	return toPrimitiveTransactionState(l.callbacks.CheckLocal(context.Background(), toMessageExt(message)))
}

func fromPrimitiveMessage(message *primitive.Message) Message {
	if message == nil {
		return Message{}
	}
	return Message{
		Topic:      message.Topic,
		Tag:        message.GetTags(),
		Key:        message.GetKeys(),
		Body:       message.Body,
		Properties: message.GetProperties(),
	}
}

func toPrimitiveTransactionState(state TransactionState) primitive.LocalTransactionState {
	switch state {
	case CommitTransaction:
		return primitive.CommitMessageState
	case RollbackTransaction:
		return primitive.RollbackMessageState
	default:
		return primitive.UnknowState
	}
}
