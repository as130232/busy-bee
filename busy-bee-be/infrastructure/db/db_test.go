package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DB_URL")
	if url == "" {
		url = "postgres://busybee:busybee@localhost:5432/busybee?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := New(ctx, url)
	if err != nil {
		t.Skipf("local postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func tempTable(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	name := fmt.Sprintf("withtx_test_%d", time.Now().UnixNano())
	_, err := pool.Exec(context.Background(),
		fmt.Sprintf("CREATE TABLE %s (v int)", name))
	if err != nil {
		t.Fatalf("create temp table: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", name))
	})
	return name
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestNew_PingFailsOnBadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := New(ctx, "postgres://nobody:wrong@localhost:59999/nope")
	if err == nil {
		t.Fatal("New() with unreachable DB should return error")
	}
}

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	pool := testPool(t)
	table := tempTable(t, pool)

	err := WithTx(context.Background(), pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			fmt.Sprintf("INSERT INTO %s (v) VALUES (1)", table))
		return err
	})
	if err != nil {
		t.Fatalf("WithTx() error = %v", err)
	}

	if got := countRows(t, pool, table); got != 1 {
		t.Errorf("rows = %d, want 1 (committed)", got)
	}
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	pool := testPool(t)
	table := tempTable(t, pool)
	boom := errors.New("boom")

	err := WithTx(context.Background(), pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(),
			fmt.Sprintf("INSERT INTO %s (v) VALUES (1)", table)); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTx() error = %v, want boom", err)
	}

	if got := countRows(t, pool, table); got != 0 {
		t.Errorf("rows = %d, want 0 (rolled back)", got)
	}
}

func TestWithTx_RollsBackOnPanic(t *testing.T) {
	pool := testPool(t)
	table := tempTable(t, pool)

	func() {
		defer func() { recover() }()
		_ = WithTx(context.Background(), pool, func(tx pgx.Tx) error {
			tx.Exec(context.Background(),
				fmt.Sprintf("INSERT INTO %s (v) VALUES (1)", table))
			panic("worker crashed")
		})
	}()

	if got := countRows(t, pool, table); got != 0 {
		t.Errorf("rows = %d, want 0 (rolled back after panic)", got)
	}
}
