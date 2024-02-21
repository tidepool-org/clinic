package store

import "github.com/kelseyhightower/envconfig"

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

type Config struct {
	DatabaseName string `envconfig:"TIDEPOOL_CLINIC_DATABASE_NAME" default:"clinic"`
	Hosts        string `envconfig:"TIDEPOOL_STORE_ADDRESSES"  default:"localhost"`
	OptParams    string `envconfig:"TIDEPOOL_STORE_OPT_PARAMS"`
	Password     string `envconfig:"TIDEPOOL_STORE_PASSWORD"`
	Scheme       string `envconfig:"TIDEPOOL_STORE_SCHEME" default:"mongodb"`
	Ssl          bool   `envconfig:"TIDEPOOL_STORE_TLS"`
	User         string `envconfig:"TIDEPOOL_STORE_USERNAME"`
}

func (c *Config) GetConnectionString() (string, error) {
	var cs string
	if c.Scheme != "" {
		cs = c.Scheme + "://"
	} else {
		cs = "mongodb://"
	}

	if c.User != "" {
		cs += c.User
		if c.Password != "" {
			cs += ":"
			cs += c.Password
		}
		cs += "@"
	}

	if c.Hosts != "" {
		cs += c.Hosts
	} else {
		cs += "localhost"
	}
	cs += "/"

	if c.Ssl == true {
		cs += "?ssl=true"
	} else {
		cs += "?ssl=false"
	}

	if c.OptParams != "" {
		cs += "&"
		cs += c.OptParams
	}
	return cs, nil
}
