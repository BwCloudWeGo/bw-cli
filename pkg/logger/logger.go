package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// FileConfig controls Lumberjack file rotation.
type FileConfig struct {
	Enabled    bool   `mapstructure:"enabled" yaml:"enabled"`
	Filename   string `mapstructure:"filename" yaml:"filename"`
	MaxSizeMB  int    `mapstructure:"max_size_mb" yaml:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups" yaml:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days" yaml:"max_age_days"`
	Compress   bool   `mapstructure:"compress" yaml:"compress"`
}

// Config controls zap logger construction and default service dimensions.
type Config struct {
	Service     string     `mapstructure:"service" yaml:"service"`
	Environment string     `mapstructure:"environment" yaml:"environment"`
	Level       string     `mapstructure:"level" yaml:"level"`
	Encoding    string     `mapstructure:"encoding" yaml:"encoding"`
	File        FileConfig `mapstructure:"file" yaml:"file"`
}

// DefaultConfig returns production-friendly logging defaults for one service.
func DefaultConfig(service string) Config {
	return Config{
		Service:     service,
		Environment: "local",
		Level:       "info",
		Encoding:    "json",
		File: FileConfig{
			Enabled:    true,
			Filename:   "logs/app.log",
			MaxSizeMB:  128,
			MaxBackups: 14,
			MaxAgeDays: 7,
			Compress:   true,
		},
	}
}

// WithDailyFileName rewrites the log filename to logs/<service>-YYYY-MM-DD.log.
func WithDailyFileName(cfg Config, now time.Time) Config {
	service := cfg.Service
	if service == "" {
		service = "app"
	}
	dir := filepath.Dir(cfg.File.Filename)
	if dir == "." || dir == "" {
		dir = "logs"
	}
	cfg.File.Filename = filepath.Join(dir, fmt.Sprintf("%s-%s.log", service, now.Format("2006-01-02")))
	return cfg
}

// New creates a zap logger with configured encoding, rotation and service fields.
func New(cfg Config) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if err := level.Set(cfg.Level); err != nil {
		return nil, err
	}
	if cfg.File.Enabled {
		if err := os.MkdirAll(filepath.Dir(cfg.File.Filename), 0o755); err != nil {
			return nil, err
		}
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = func(d time.Duration, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendFloat64(float64(d.Microseconds()) / 1000)
	}

	var encoder zapcore.Encoder
	if cfg.Encoding == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(encoder, zapcore.AddSync(writer(cfg.File)), level)
	log := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return log.With(
		zap.String("service", cfg.Service),
		zap.String("env", cfg.Environment),
	), nil
}

func writer(cfg FileConfig) zapcore.WriteSyncer {
	if !cfg.Enabled {
		return zapcore.AddSync(os.Stdout)
	}
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
	})
}
