// Package db 提供 pgx 連線池與 transaction boundary helper。
// Transaction boundary 一律在 application 層透過 WithTx 建立，repository 不開 transaction。
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New 建立連線池並以 Ping 驗證連線。
func New(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("db.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db.New ping: %w", err)
	}
	return pool, nil
}

// WithTx 執行 fn 於單一 transaction：fn 回錯誤或 panic 時 rollback，否則 commit。
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db.WithTx begin: %w", err)
	}
	defer tx.Rollback(ctx) // commit 後的 rollback 是 no-op；panic 時確保回滾

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db.WithTx commit: %w", err)
	}
	return nil
}
