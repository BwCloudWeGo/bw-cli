package rocketmqx

import (
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
)

// Message 是业务层发布 RocketMQ 消息时使用的统一结构。
type Message struct {
	Topic      string
	Tag        string
	Key        string
	Body       []byte
	Properties map[string]string
	CreatedAt  time.Time
}

// DelayMessage 表示一条 RocketMQ 延时等级消息。
type DelayMessage struct {
	Message
	Level int
}

// TransactionMessage 表示一条 RocketMQ 事务消息。
type TransactionMessage struct {
	Message
}

// MessageExt 是消费侧暴露给业务层的消息结构。
type MessageExt struct {
	Message
	MessageID      string
	ReconsumeTimes int32
	QueueOffset    int64
	StoreAt        time.Time
}

// SendResult 是生产者发布后的稳定结果结构。
type SendResult struct {
	MessageID     string
	Status        string
	TransactionID string
}

func toPrimitiveMessage(message Message) *primitive.Message {
	msg := primitive.NewMessage(message.Topic, message.Body)
	if message.Tag != "" {
		msg.WithTag(message.Tag)
	}
	if message.Key != "" {
		msg.WithKeys([]string{message.Key})
	}
	for key, value := range message.Properties {
		msg.WithProperty(key, value)
	}
	return msg
}

func toDelayPrimitiveMessage(message DelayMessage) *primitive.Message {
	msg := toPrimitiveMessage(message.Message)
	if message.Level > 0 {
		msg.WithDelayTimeLevel(message.Level)
	}
	return msg
}

func toMessageExt(message *primitive.MessageExt) MessageExt {
	if message == nil {
		return MessageExt{}
	}
	return MessageExt{
		Message: Message{
			Topic:      message.Topic,
			Tag:        message.GetTags(),
			Key:        message.GetKeys(),
			Body:       message.Body,
			Properties: message.GetProperties(),
			CreatedAt:  unixMillis(message.BornTimestamp),
		},
		MessageID:      message.MsgId,
		ReconsumeTimes: message.ReconsumeTimes,
		QueueOffset:    message.QueueOffset,
		StoreAt:        unixMillis(message.StoreTimestamp),
	}
}

func toSendResult(result *primitive.SendResult) SendResult {
	if result == nil {
		return SendResult{}
	}
	return SendResult{
		MessageID:     result.MsgID,
		Status:        sendStatus(result.Status),
		TransactionID: result.TransactionID,
	}
}

func sendStatus(status primitive.SendStatus) string {
	switch status {
	case primitive.SendOK:
		return "send_ok"
	case primitive.SendFlushDiskTimeout:
		return "flush_disk_timeout"
	case primitive.SendFlushSlaveTimeout:
		return "flush_slave_timeout"
	case primitive.SendSlaveNotAvailable:
		return "slave_not_available"
	default:
		return "unknown"
	}
}

func unixMillis(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(value)
}
