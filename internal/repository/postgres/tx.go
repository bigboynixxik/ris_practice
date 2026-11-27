package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// TxManager provides the transaction boundary requested by the service.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager constructs a transaction manager for the application's pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx reuses an existing transaction or commits all successful changes together.
func (tm *TxManager) WithTx(ctx context.Context, fn func(context.Context) error) (runErr error) {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("TxManager.WithTx begin: %w", err)
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			runErr = errors.Join(runErr, fmt.Errorf("TxManager.WithTx rollback: %w", err))
		}
	}()
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("TxManager.WithTx commit: %w", err)
	}
	return nil
}
