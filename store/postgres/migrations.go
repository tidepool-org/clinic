package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all pending schema migrations to the configured database.
// A session-level advisory lock makes concurrent invocations (e.g. multiple
// replicas starting simultaneously) safe.
func Migrate(ctx context.Context, config *Config) error {
	db, err := sql.Open("pgx", config.ConnectionString())
	if err != nil {
		return fmt.Errorf("unable to open postgres connection for migrations: %w", err)
	}
	defer db.Close()

	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return err
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations, goose.WithSessionLocker(sessionLocker))
	if err != nil {
		return fmt.Errorf("unable to create goose provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("unable to apply postgres migrations: %w", err)
	}
	return nil
}
