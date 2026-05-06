package config

// GlobalConfig holds the process-wide runtime configuration after InitGlobal succeeds.
// It is initialized once in main and can be read by packages that need shared config.
var GlobalConfig *Config

// InitGlobal loads configuration from YAML/env and stores it as the process-wide config.
func InitGlobal(path string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	GlobalConfig = cfg
	return nil
}

// MustGlobal returns the process-wide config or panics if InitGlobal was not called.
func MustGlobal() *Config {
	if GlobalConfig == nil {
		panic("global config is not initialized")
	}
	return GlobalConfig
}
