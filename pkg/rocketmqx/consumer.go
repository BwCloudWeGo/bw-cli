package rocketmqx

import (
	"context"
	"errors"
	"strings"

	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// ConsumeHandler 是业务层处理 RocketMQ 消息的函数。
type ConsumeHandler func(context.Context, MessageExt) error

// Subscription 描述一个 RocketMQ 订阅。
type Subscription struct {
	Topic string
	Tags  []string
}

// PushConsumerClient 是 PushConsumer 依赖的最小 SDK 接口。
type PushConsumerClient interface {
	Start() error
	Shutdown() error
	Subscribe(string, consumer.MessageSelector, func(context.Context, ...*primitive.MessageExt) (consumer.ConsumeResult, error)) error
}

// PushConsumer 封装 RocketMQ push 消费者。
type PushConsumer struct {
	client PushConsumerClient
}

// NewPushConsumer 创建并返回 RocketMQ push 消费者。
func NewPushConsumer(cfg Config) (*PushConsumer, error) {
	normalized := cfg.Normalize()
	options := []consumer.Option{
		consumer.WithNameServer(primitive.NamesrvAddr(normalized.NameServers)),
		consumer.WithGroupName(normalized.ConsumerGroup),
		consumer.WithConsumeMessageBatchMaxSize(normalized.ConsumeMessageBatchMaxSize),
	}
	if normalized.Namespace != "" {
		options = append(options, consumer.WithNamespace(normalized.Namespace))
	}
	if normalized.AccessKey != "" || normalized.SecretKey != "" {
		options = append(options, consumer.WithCredentials(primitive.Credentials{
			AccessKey: normalized.AccessKey,
			SecretKey: normalized.SecretKey,
		}))
	}
	client, err := consumer.NewPushConsumer(options...)
	if err != nil {
		return nil, err
	}
	return NewPushConsumerWithClient(client), nil
}

// NewPushConsumerWithClient 基于已有客户端创建 push 消费者。
func NewPushConsumerWithClient(client PushConsumerClient) *PushConsumer {
	return &PushConsumer{client: client}
}

// Subscribe 订阅消息，并在 handler 返回 nil 后确认消费成功。
func (c *PushConsumer) Subscribe(_ context.Context, subscription Subscription, handler ConsumeHandler) error {
	if c == nil || c.client == nil {
		return errors.New("rocketmq push consumer client is nil")
	}
	if handler == nil {
		return errors.New("rocketmq consume handler is nil")
	}
	selector := consumer.MessageSelector{
		Type:       consumer.TAG,
		Expression: tagExpression(subscription.Tags),
	}
	if err := c.client.Subscribe(subscription.Topic, selector, func(ctx context.Context, messages ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
		for _, message := range messages {
			if err := handler(ctx, toMessageExt(message)); err != nil {
				return consumer.ConsumeRetryLater, err
			}
		}
		return consumer.ConsumeSuccess, nil
	}); err != nil {
		return err
	}
	return c.client.Start()
}

// Close 关闭底层 RocketMQ push 消费者。
func (c *PushConsumer) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Shutdown()
}

func tagExpression(tags []string) string {
	cleaned := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleaned = append(cleaned, tag)
		}
	}
	if len(cleaned) == 0 {
		return "*"
	}
	return strings.Join(cleaned, " || ")
}
