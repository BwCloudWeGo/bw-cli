package observability

import "go.uber.org/zap"

// Register 是指标和链路追踪导出器的扩展点。
func Register(service string, log *zap.Logger) {
	log.Info("observability hooks registered", zap.String("service", service))
}
