// Package app wires the commands and their dependencies.
package app

import (
	"context"
	"fmt"

	"ris/internal/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const defaultDSN = "postgres://ris:ris@127.0.0.1:55432/ris?sslmode=disable"

func openDatabase(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("app.openDatabase configure database pool: %w", err)
	}
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("app.openDatabase apply migrations: %w", err)
	}
	return pool, nil
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		return fmt.Errorf("app.migrate initialize Goose: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("app.migrate run Goose migrations: %w", err)
	}
	return nil
}
