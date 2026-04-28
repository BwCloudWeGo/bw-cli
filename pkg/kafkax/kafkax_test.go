package kafkax_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/kafkax"
)

func TestDefaultConfig(t *testing.T) {
	cfg := kafkax.DefaultConfig()

	require.Equal(t, []string{"127.0.0.1:9092"}, cfg.Brokers)
	require.Equal(t, "xiaolanshu-events", cfg.Topic)
	require.Equal(t, "xiaolanshu-consumer", cfg.GroupID)
}
