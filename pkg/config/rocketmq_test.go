package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadRocketMQConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config.yaml", `
rocketmq:
  name_servers:
    - 10.0.0.1:9876
  group_name: order-producer
  consumer_group: order-consumer
  namespace: order
  access_key: ""
  secret_key: ""
  retry_times: 4
  send_timeout: 5s
  consume_message_batch_max_size: 8
`)

	cfg, err := Load(path)

	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1:9876"}, cfg.RocketMQ.NameServers)
	require.Equal(t, "order-producer", cfg.RocketMQ.GroupName)
	require.Equal(t, "order-consumer", cfg.RocketMQ.ConsumerGroup)
	require.Equal(t, "order", cfg.RocketMQ.Namespace)
	require.Equal(t, 4, cfg.RocketMQ.RetryTimes)
	require.Equal(t, 5*time.Second, cfg.RocketMQ.SendTimeout)
	require.Equal(t, 8, cfg.RocketMQ.ConsumeMessageBatchMaxSize)
}
