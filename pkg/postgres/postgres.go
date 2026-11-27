// Package postgres configures the PostgreSQL connection pool.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxConnections = 64
	maxLifetime    = time.Hour
	maxIdleTime    = 30 * time.Minute
	pingTimeout    = 5 * time.Second
)

// NewPool parses the DSN, opens a pool and verifies connectivity.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres.NewPool parse DSN: %w", err)
	}
	config.MaxConns = maxConnections
	config.MaxConnLifetime = maxLifetime
	config.MaxConnIdleTime = maxIdleTime
	config.PingTimeout = pingTimeout

	connectCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectCtx, config)
	if err != nil {
		return nil, fmt.Errorf("postgres.NewPool create pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres.NewPool ping database: %w", err)
	}
	return pool, nil
}
