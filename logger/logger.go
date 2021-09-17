package logger

import "go.uber.org/zap"

func NewProductionLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	return config.Build()
}

func Suggar(logger *zap.Logger) *zap.SugaredLogger {
	return logger.Sugar()
}
