// Package migrator applies embedded PostgreSQL schema migrations with Goose.
package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"ris/pkg/logger"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrator applies the schema to a PostgreSQL database.
type Migrator struct {
	db         *sql.DB
	migrations fs.FS
}

// New creates a migrator from an embedded filesystem or another fs.FS.
func New(db *sql.DB, migrations fs.FS) *Migrator {
	return &Migrator{db: db, migrations: migrations}
}

// UpContext applies pending migrations and serializes concurrent migration runs.
func (m *Migrator) UpContext(ctx context.Context) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("migrator.UpContext configure lock: %w", err)
	}
	// Keep the default Goose history table used by the original importer.
	runner, err := goose.NewProvider(goose.DialectPostgres, m.db, m.migrations,
		goose.WithSessionLocker(locker),
		goose.WithSlog(logger.FromContext(ctx)),
	)
	if err != nil {
		return fmt.Errorf("migrator.UpContext configure Goose: %w", err)
	}
	if _, err := runner.Up(ctx); err != nil {
		return fmt.Errorf("migrator.UpContext apply migrations: %w", err)
	}
	return nil
}
