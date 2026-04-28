package postgresx

import (
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Config controls PostgreSQL connection creation and sql.DB pool settings.
type Config struct {
	DSN             string              `mapstructure:"dsn" yaml:"dsn"`
	MaxIdleConns    int                 `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns    int                 `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	ConnMaxLifetime time.Duration       `mapstructure:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	LogLevel        gormlogger.LogLevel `mapstructure:"log_level" yaml:"log_level"`
}

// DefaultConfig returns safe pool defaults without a DSN.
func DefaultConfig() Config {
	return Config{
		MaxIdleConns:    10,
		MaxOpenConns:    100,
		ConnMaxLifetime: time.Hour,
		LogLevel:        gormlogger.Warn,
	}
}

// Open creates a Gorm PostgreSQL connection using only caller-provided DSN values.
func Open(cfg Config) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, errors.New("postgres dsn is required")
	}
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(cfg.LogLevel),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	return db, nil
}
