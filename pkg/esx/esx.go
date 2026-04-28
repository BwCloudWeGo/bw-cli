package esx

import (
	"github.com/elastic/go-elasticsearch/v8"
)

// Config controls Elasticsearch client connection settings.
type Config struct {
	Addresses []string `mapstructure:"addresses" yaml:"addresses"`
	Username  string   `mapstructure:"username" yaml:"username"`
	Password  string   `mapstructure:"password" yaml:"password"`
	CloudID   string   `mapstructure:"cloud_id" yaml:"cloud_id"`
	APIKey    string   `mapstructure:"api_key" yaml:"api_key"`
}

// DefaultConfig returns local-development Elasticsearch defaults.
func DefaultConfig() Config {
	return Config{
		Addresses: []string{"http://127.0.0.1:9200"},
	}
}

// NewClient creates an Elasticsearch v8 client from configuration.
func NewClient(cfg Config) (*elasticsearch.Client, error) {
	if len(cfg.Addresses) == 0 && cfg.CloudID == "" {
		cfg.Addresses = DefaultConfig().Addresses
	}
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		CloudID:   cfg.CloudID,
		APIKey:    cfg.APIKey,
	})
}
