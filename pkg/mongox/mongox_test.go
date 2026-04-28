package mongox_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
)

func TestDefaultConfigUsesLocalDevelopmentValues(t *testing.T) {
	cfg := mongox.DefaultConfig()

	require.Equal(t, "mongodb://127.0.0.1:27017", cfg.URI)
	require.Equal(t, "xiaolanshu", cfg.Database)
	require.Equal(t, "bw-cli", cfg.AppName)
	require.Equal(t, uint64(0), cfg.MinPoolSize)
	require.Equal(t, uint64(100), cfg.MaxPoolSize)
	require.Equal(t, 10*time.Second, cfg.ConnectTimeout)
	require.Equal(t, 5*time.Second, cfg.ServerSelectionTimeout)
}

func TestNewClientFallsBackToDefaultURI(t *testing.T) {
	client, err := mongox.NewClient(mongox.Config{})
	require.NoError(t, err)
	require.NotNil(t, client)

	require.NoError(t, client.Disconnect(context.Background()))
}

func TestDatabaseReturnsConfiguredDatabase(t *testing.T) {
	client, err := mongox.NewClient(mongox.Config{URI: "mongodb://127.0.0.1:27017", Database: "content"})
	require.NoError(t, err)
	defer client.Disconnect(context.Background())

	db := mongox.Database(client, "content")

	require.Equal(t, "content", db.Name())
}
