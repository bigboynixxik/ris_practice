package app

import (
	"context"
	"errors"
	"fmt"

	"ris/internal/migrations"
	"ris/pkg/migrator"
	"ris/pkg/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func openDatabase(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("app.openDatabase: %w", err)
	}

	db := stdlib.OpenDBFromPool(pool)
	migrationErr := migrator.New(db, migrations.Files).UpContext(ctx)
	closeErr := db.Close()
	migrationErr = errors.Join(migrationErr, closeErr)
	if migrationErr != nil {
		pool.Close()
		return nil, fmt.Errorf("app.openDatabase apply migrations: %w", migrationErr)
	}
	return pool, nil
}
