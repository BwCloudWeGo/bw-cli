package rocketmqx

import (
	"context"
	"errors"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// SyncSender 是简单消息和延时消息生产者依赖的最小 SDK 接口。
type SyncSender interface {
	Start() error
	Shutdown() error
	SendSync(context.Context, ...*primitive.Message) (*primitive.SendResult, error)
}

// SimpleProducer 发布普通 RocketMQ 消息。
type SimpleProducer struct {
	sender SyncSender
}

// NewSimpleProducer 创建并启动普通消息生产者。
func NewSimpleProducer(cfg Config) (*SimpleProducer, error) {
	sender, err := producer.NewDefaultProducer(cfg.producerOptions()...)
	if err != nil {
		return nil, err
	}
	p := NewSimpleProducerWithSender(sender)
	if err := p.Start(); err != nil {
		return nil, err
	}
	return p, nil
}

// NewSimpleProducerWithSender 基于已有发送器创建普通消息生产者，主要用于测试和自定义 SDK。
func NewSimpleProducerWithSender(sender SyncSender) *SimpleProducer {
	return &SimpleProducer{sender: sender}
}

// Start 启动底层 RocketMQ 生产者。
func (p *SimpleProducer) Start() error {
	if p == nil || p.sender == nil {
		return errors.New("rocketmq simple producer sender is nil")
	}
	return p.sender.Start()
}

// Publish 同步发布一条普通消息。
func (p *SimpleProducer) Publish(ctx context.Context, message Message) (SendResult, error) {
	if p == nil || p.sender == nil {
		return SendResult{}, errors.New("rocketmq simple producer sender is nil")
	}
	result, err := p.sender.SendSync(ctx, toPrimitiveMessage(message))
	if err != nil {
		return SendResult{}, err
	}
	return toSendResult(result), nil
}

// Close 关闭底层 RocketMQ 生产者。
func (p *SimpleProducer) Close() error {
	if p == nil || p.sender == nil {
		return nil
	}
	return p.sender.Shutdown()
}

// DelayProducer 发布 RocketMQ 延时等级消息。
type DelayProducer struct {
	sender SyncSender
}

// NewDelayProducer 创建并启动延时消息生产者。
func NewDelayProducer(cfg Config) (*DelayProducer, error) {
	sender, err := producer.NewDefaultProducer(cfg.producerOptions()...)
	if err != nil {
		return nil, err
	}
	p := NewDelayProducerWithSender(sender)
	if err := p.Start(); err != nil {
		return nil, err
	}
	return p, nil
}

// NewDelayProducerWithSender 基于已有发送器创建延时消息生产者。
func NewDelayProducerWithSender(sender SyncSender) *DelayProducer {
	return &DelayProducer{sender: sender}
}

// Start 启动底层 RocketMQ 生产者。
func (p *DelayProducer) Start() error {
	if p == nil || p.sender == nil {
		return errors.New("rocketmq delay producer sender is nil")
	}
	return p.sender.Start()
}

// PublishDelay 同步发布一条延时消息。
func (p *DelayProducer) PublishDelay(ctx context.Context, message DelayMessage) (SendResult, error) {
	if p == nil || p.sender == nil {
		return SendResult{}, errors.New("rocketmq delay producer sender is nil")
	}
	result, err := p.sender.SendSync(ctx, toDelayPrimitiveMessage(message))
	if err != nil {
		return SendResult{}, err
	}
	return toSendResult(result), nil
}

// Close 关闭底层 RocketMQ 生产者。
func (p *DelayProducer) Close() error {
	if p == nil || p.sender == nil {
		return nil
	}
	return p.sender.Shutdown()
}
