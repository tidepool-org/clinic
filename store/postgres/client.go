package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Client wraps the pgx connection pool. When dual writes are disabled the
// client carries a nil pool and Enabled() returns false; consumers must not
// touch the pool in that case.
type Client struct {
	config *Config
	pool   *pgxpool.Pool
}

func NewClient(config *Config, logger *zap.SugaredLogger, lc fx.Lifecycle) (*Client, error) {
	client := &Client{config: config}
	if !config.Enabled {
		logger.Info("postgres dual writes are disabled")
		return client, nil
	}

	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("unable to parse postgres connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create postgres pool: %w", err)
	}
	client.pool = pool

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("unable to connect to postgres: %w", err)
			}
			if config.RunMigrations {
				if err := Migrate(ctx, config); err != nil {
					return err
				}
			}
			logger.Infow("postgres dual writes are enabled", "database", config.DatabaseName)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})

	return client, nil
}

func (c *Client) Enabled() bool {
	return c != nil && c.pool != nil
}

// Pool returns the underlying connection pool. It is nil when dual writes
// are disabled; guard with Enabled().
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}
