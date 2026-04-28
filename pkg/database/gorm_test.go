package database_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/database"
)

func TestMySQLConfigUsesValuesFromAppConfig(t *testing.T) {
	cfg := config.MySQLConfig{
		DSN:                    "user:pass@tcp(mysql:3306)/app?parseTime=True",
		MaxIdleConns:           3,
		MaxOpenConns:           17,
		ConnMaxLifetimeSeconds: 45,
	}

	mysqlCfg := database.ToMySQLConfig(cfg)

	require.Equal(t, cfg.DSN, mysqlCfg.DSN)
	require.Equal(t, 3, mysqlCfg.MaxIdleConns)
	require.Equal(t, 17, mysqlCfg.MaxOpenConns)
	require.Equal(t, 45*time.Second, mysqlCfg.ConnMaxLifetime)
}
