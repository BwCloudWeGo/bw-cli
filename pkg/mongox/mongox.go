package mongox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Config controls MongoDB client connection, database selection and pool settings.
type Config struct {
	URI                    string        `mapstructure:"uri" yaml:"uri"`
	Database               string        `mapstructure:"database" yaml:"database"`
	AppName                string        `mapstructure:"app_name" yaml:"app_name"`
	MinPoolSize            uint64        `mapstructure:"min_pool_size" yaml:"min_pool_size"`
	MaxPoolSize            uint64        `mapstructure:"max_pool_size" yaml:"max_pool_size"`
	ConnectTimeout         time.Duration `mapstructure:"connect_timeout" yaml:"connect_timeout"`
	ServerSelectionTimeout time.Duration `mapstructure:"server_selection_timeout" yaml:"server_selection_timeout"`
}

// DefaultConfig returns local-development MongoDB defaults.
func DefaultConfig() Config {
	return Config{
		URI:                    "mongodb://127.0.0.1:27017",
		Database:               "xiaolanshu",
		AppName:                "bw-cli",
		MinPoolSize:            0,
		MaxPoolSize:            100,
		ConnectTimeout:         10 * time.Second,
		ServerSelectionTimeout: 5 * time.Second,
	}
}

// NewClient creates a MongoDB client from configuration without forcing a network ping.
func NewClient(cfg Config) (*mongo.Client, error) {
	defaults := DefaultConfig()
	if cfg.URI == "" {
		cfg.URI = defaults.URI
	}
	if cfg.Database == "" {
		cfg.Database = defaults.Database
	}
	if cfg.AppName == "" {
		cfg.AppName = defaults.AppName
	}
	if cfg.MaxPoolSize == 0 {
		cfg.MaxPoolSize = defaults.MaxPoolSize
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}
	if cfg.ServerSelectionTimeout == 0 {
		cfg.ServerSelectionTimeout = defaults.ServerSelectionTimeout
	}

	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetAppName(cfg.AppName).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetServerSelectionTimeout(cfg.ServerSelectionTimeout)
	return mongo.Connect(clientOptions)
}

// Database returns the configured database handle from an existing MongoDB client.
func Database(client *mongo.Client, name string) *mongo.Database {
	return client.Database(name)
}

// Ping checks that the MongoDB client can reach a primary or secondary node.
func Ping(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, readpref.PrimaryPreferred())
}
