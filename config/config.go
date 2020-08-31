package config

import "github.com/kelseyhightower/envconfig"

type Config struct {
	GrpcPort              uint16 `envconfig:"TIDEPOOL_GRPC_SERVER_PORT" default:"50051" required:"true"`
	HttpPort              uint16 `envconfig:"TIDEPOOL_HTTP_SERVER_PORT" default:"8080" required:"true"`
}

func New() *Config {
	return &Config{}
}

func (c *Config) LoadFromEnv() error {
	return envconfig.Process("", c)
}
