package rocketmqx

import (
	"strings"
	"time"

	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/apache/rocketmq-client-go/v2/producer"
)

// Config 控制 RocketMQ 生产者和消费者的创建。
type Config struct {
	NameServers                []string      `mapstructure:"name_servers" yaml:"name_servers"`
	GroupName                  string        `mapstructure:"group_name" yaml:"group_name"`
	ConsumerGroup              string        `mapstructure:"consumer_group" yaml:"consumer_group"`
	Namespace                  string        `mapstructure:"namespace" yaml:"namespace"`
	AccessKey                  string        `mapstructure:"access_key" yaml:"access_key"`
	SecretKey                  string        `mapstructure:"secret_key" yaml:"secret_key"`
	RetryTimes                 int           `mapstructure:"retry_times" yaml:"retry_times"`
	SendTimeout                time.Duration `mapstructure:"send_timeout" yaml:"send_timeout"`
	ConsumeMessageBatchMaxSize int           `mapstructure:"consume_message_batch_max_size" yaml:"consume_message_batch_max_size"`
}

// DefaultConfig 返回本地开发可用的 RocketMQ 默认配置。
func DefaultConfig() Config {
	return Config{
		NameServers:                []string{"127.0.0.1:9876"},
		GroupName:                  "xiaolanshu-producer",
		ConsumerGroup:              "xiaolanshu-consumer",
		RetryTimes:                 2,
		SendTimeout:                3 * time.Second,
		ConsumeMessageBatchMaxSize: 1,
	}
}

// Normalize 在保留调用方配置的同时补齐默认值。
func (cfg Config) Normalize() Config {
	defaults := DefaultConfig()
	if len(cfg.NameServers) == 0 {
		cfg.NameServers = defaults.NameServers
	}
	if strings.TrimSpace(cfg.GroupName) == "" {
		cfg.GroupName = defaults.GroupName
	}
	if strings.TrimSpace(cfg.ConsumerGroup) == "" {
		cfg.ConsumerGroup = defaults.ConsumerGroup
	}
	if cfg.RetryTimes == 0 {
		cfg.RetryTimes = defaults.RetryTimes
	}
	if cfg.SendTimeout == 0 {
		cfg.SendTimeout = defaults.SendTimeout
	}
	if cfg.ConsumeMessageBatchMaxSize == 0 {
		cfg.ConsumeMessageBatchMaxSize = defaults.ConsumeMessageBatchMaxSize
	}
	return cfg
}

func (cfg Config) producerOptions() []producer.Option {
	cfg = cfg.Normalize()
	options := []producer.Option{
		producer.WithNameServer(primitive.NamesrvAddr(cfg.NameServers)),
		producer.WithGroupName(cfg.GroupName),
		producer.WithRetry(cfg.RetryTimes),
		producer.WithSendMsgTimeout(cfg.SendTimeout),
	}
	if cfg.Namespace != "" {
		options = append(options, producer.WithNamespace(cfg.Namespace))
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		options = append(options, producer.WithCredentials(primitive.Credentials{
			AccessKey: cfg.AccessKey,
			SecretKey: cfg.SecretKey,
		}))
	}
	return options
}
