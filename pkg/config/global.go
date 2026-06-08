package config

// GlobalConfig 在 InitGlobal 成功后保存进程级运行时配置。
// 它在 main 中初始化一次，供需要共享配置的包读取。
var GlobalConfig *Config

// InitGlobal 加载配置并保存为进程级配置。
func InitGlobal(path string) error {
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	GlobalConfig = cfg
	return nil
}

// MustGlobal 返回进程级配置；若未调用 InitGlobal 则 panic。
func MustGlobal() *Config {
	if GlobalConfig == nil {
		panic("global config is not initialized")
	}
	return GlobalConfig
}
