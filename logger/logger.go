package logger

import (
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Config struct {
	Level string `envconfig:"LOG_LEVEL" default:"debug"`
}

func NewProductionLogger() (*zap.Logger, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	config := zap.NewProductionConfig()
	config.Level = level
	return config.Build()
}

func Suggar(logger *zap.Logger) *zap.SugaredLogger {
	return logger.Sugar()
}
