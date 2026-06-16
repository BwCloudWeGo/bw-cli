# RocketMQ 调用示例

本文档说明如何使用 `pkg/rocketmqx` 在业务 service 中发布和订阅 RocketMQ 消息。底层 SDK 使用 `github.com/apache/rocketmq-client-go/v2`。

## 1. 配置

默认配置写在 `configs/config.yaml`。如果启用了 Nacos，把完整业务 YAML 同步到 Nacos 后即可读取同样结构。

```yaml
rocketmq:
  name_servers:
    - 127.0.0.1:9876
  group_name: xiaolanshu-producer
  consumer_group: xiaolanshu-consumer
  namespace: ""
  access_key: ""
  secret_key: ""
  retry_times: 2
  send_timeout: 3s
  consume_message_batch_max_size: 1
```

## 2. 普通消息

进程入口初始化一次生产者，然后注入业务 service：

```go
producer, err := rocketmqx.NewSimpleProducer(cfg.RocketMQ)
if err != nil {
    return err
}
defer producer.Close()

noteSvc := noteservice.NewService(repo, producer, log)
```

业务 service 直接发布：

```go
func (s *Service) PublishNoteCreated(ctx context.Context, noteID string, authorID string) error {
    payload, err := json.Marshal(map[string]string{
        "event":     "note.created",
        "note_id":   noteID,
        "author_id": authorID,
    })
    if err != nil {
        return err
    }

    _, err = s.producer.Publish(ctx, rocketmqx.Message{
        Topic: "note-events",
        Tag:   "note.created",
        Key:   noteID,
        Body:  payload,
        Properties: map[string]string{
            "source": "note-service",
        },
    })
    return err
}
```

## 3. 延时消息

RocketMQ 4.x 延时消息使用固定 delay level。常见等级由 broker 配置决定，默认通常是 `1s 5s 10s 30s 1m ...`。

```go
delayProducer, err := rocketmqx.NewDelayProducer(cfg.RocketMQ)
if err != nil {
    return err
}
defer delayProducer.Close()

_, err = delayProducer.PublishDelay(ctx, rocketmqx.DelayMessage{
    Message: rocketmqx.Message{
        Topic: "order-events",
        Tag:   "order.timeout",
        Key:   orderID,
        Body:  payload,
    },
    Level: 3,
})
```

## 4. 事务消息

事务消息适合“先发半消息，再执行本地事务，最后提交或回滚”的场景。

```go
txProducer, err := rocketmqx.NewTransactionProducer(cfg.RocketMQ, rocketmqx.TransactionCallbacks{
    ExecuteLocal: func(ctx context.Context, msg rocketmqx.Message) (rocketmqx.TransactionState, error) {
        if err := orderRepo.MarkPaid(ctx, orderID); err != nil {
            return rocketmqx.RollbackTransaction, err
        }
        return rocketmqx.CommitTransaction, nil
    },
    CheckLocal: func(ctx context.Context, msg rocketmqx.MessageExt) rocketmqx.TransactionState {
        if orderRepo.IsPaid(ctx, orderID) {
            return rocketmqx.CommitTransaction
        }
        return rocketmqx.RollbackTransaction
    },
})
if err != nil {
    return err
}
defer txProducer.Close()

_, err = txProducer.PublishTransaction(ctx, rocketmqx.TransactionMessage{
    Message: rocketmqx.Message{
        Topic: "payment-events",
        Tag:   "payment.paid",
        Key:   orderID,
        Body:  payload,
    },
})
```

## 5. 消费者

消费者适合在服务启动时注册订阅。handler 返回 `nil` 表示消费成功，返回 error 会让 RocketMQ 稍后重试。

```go
consumer, err := rocketmqx.NewPushConsumer(cfg.RocketMQ)
if err != nil {
    return err
}
defer consumer.Close()

err = consumer.Subscribe(ctx, rocketmqx.Subscription{
    Topic: "note-events",
    Tags:  []string{"note.created", "note.updated"},
}, func(ctx context.Context, msg rocketmqx.MessageExt) error {
    return noteSvc.HandleNoteEvent(ctx, msg.Tag, msg.Body)
})
```

业务分层建议：

```text
cmd/<service>/main.go -> 初始化 producer/consumer
service              -> 直接调用 rocketmqx.Producer 或处理 ConsumeHandler
repo                 -> 只处理数据库
handler              -> 不直接操作 RocketMQ
```

