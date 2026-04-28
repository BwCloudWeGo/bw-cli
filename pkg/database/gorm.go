package database

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"github.com/BwCloudWeGo/bw-cli/pkg/mysqlx"
)

// Open creates a Gorm database connection from the service configuration.
func Open(cfg config.DatabaseConfig, mysqlCfg config.MySQLConfig, log *zap.Logger) (*gorm.DB, error) {
	if cfg.Driver == "mysql" {
		db, err := mysqlx.Open(ToMySQLConfig(mysqlCfg))
		if err != nil {
			return nil, err
		}
		log.Info("database connected", zap.String("driver", cfg.Driver))
		return db, nil
	}
	if cfg.Driver != "sqlite" {
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
	if err := ensureDir(cfg.DSN); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, err
	}
	log.Info("database connected", zap.String("driver", cfg.Driver), zap.String("dsn", cfg.DSN))
	return db, nil
}

// ToMySQLConfig converts application YAML config into the reusable mysqlx config.
func ToMySQLConfig(cfg config.MySQLConfig) mysqlx.Config {
	return mysqlx.Config{
		DSN:             cfg.DSN,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxOpenConns:    cfg.MaxOpenConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime(),
		LogLevel:        gormlogger.Warn,
	}
}

// ensureDir creates the directory for file-based SQLite DSNs.
func ensureDir(dsn string) error {
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
