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
	"github.com/BwCloudWeGo/bw-cli/pkg/postgresx"
)

// Open 根据服务配置创建 Gorm 数据库连接。
func Open(cfg config.DatabaseConfig, mysqlCfg config.MySQLConfig, postgresCfg config.PostgreSQLConfig, log *zap.Logger) (*gorm.DB, error) {
	switch cfg.Driver {
	case "mysql":
		db, err := mysqlx.Open(ToMySQLConfig(mysqlCfg))
		if err != nil {
			return nil, err
		}
		log.Info("database connected", zap.String("driver", cfg.Driver))
		return db, nil
	case "postgres", "postgresql", "pg":
		db, err := postgresx.Open(ToPostgreSQLConfig(postgresCfg))
		if err != nil {
			return nil, err
		}
		log.Info("database connected", zap.String("driver", cfg.Driver))
		return db, nil
	case "sqlite":
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
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

// ToMySQLConfig 将应用 YAML 配置转换为可复用的 mysqlx 配置。
func ToMySQLConfig(cfg config.MySQLConfig) mysqlx.Config {
	return mysqlx.Config{
		DSN:             cfg.DSN,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxOpenConns:    cfg.MaxOpenConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime(),
		LogLevel:        gormlogger.Warn,
	}
}

// ToPostgreSQLConfig 将应用 YAML 配置转换为可复用的 postgresx 配置。
func ToPostgreSQLConfig(cfg config.PostgreSQLConfig) postgresx.Config {
	return postgresx.Config{
		DSN:             cfg.DSN,
		MaxIdleConns:    cfg.MaxIdleConns,
		MaxOpenConns:    cfg.MaxOpenConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime(),
		LogLevel:        gormlogger.Warn,
	}
}

// ensureDir 为基于文件的 SQLite DSN 创建目录。
func ensureDir(dsn string) error {
	dir := filepath.Dir(dsn)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
