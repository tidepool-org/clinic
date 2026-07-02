package postgres

import (
	"fmt"
	"net/url"

	"github.com/kelseyhightower/envconfig"
)

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Config configures the PostgreSQL connection used for dual writes while
// MongoDB remains the source of truth. When Enabled is false the service
// doesn't connect to PostgreSQL at all and every repository operates
// against MongoDB only.
type Config struct {
	Enabled       bool   `envconfig:"TIDEPOOL_POSTGRES_ENABLED" default:"false"`
	Host          string `envconfig:"TIDEPOOL_POSTGRES_HOST" default:"localhost"`
	Port          int    `envconfig:"TIDEPOOL_POSTGRES_PORT" default:"5432"`
	User          string `envconfig:"TIDEPOOL_POSTGRES_USERNAME" default:"postgres"`
	Password      string `envconfig:"TIDEPOOL_POSTGRES_PASSWORD"`
	DatabaseName  string `envconfig:"TIDEPOOL_POSTGRES_DATABASE_NAME" default:"clinic"`
	SslMode       string `envconfig:"TIDEPOOL_POSTGRES_SSL_MODE" default:"disable"`
	RunMigrations bool   `envconfig:"TIDEPOOL_POSTGRES_RUN_MIGRATIONS" default:"true"`
}

func (c *Config) ConnectionString() string {
	u := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:   "/" + c.DatabaseName,
	}
	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}
	query := url.Values{}
	if c.SslMode != "" {
		query.Set("sslmode", c.SslMode)
	}
	u.RawQuery = query.Encode()
	return u.String()
}
