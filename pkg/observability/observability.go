package observability

import "go.uber.org/zap"

// Register is the extension point for metrics and tracing exporters.
func Register(service string, log *zap.Logger) {
	log.Info("observability hooks registered", zap.String("service", service))
}
