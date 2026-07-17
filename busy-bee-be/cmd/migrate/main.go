// migrate 執行 embedded DB migrations。
// 用法：go run ./cmd/migrate [up|down|version]（預設 up）
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/as130232/busy-bee/busy-bee-be/db/migrations"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DB.URL == "" {
		return errors.New("DB_URL is required")
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, "pgx5://"+trimScheme(cfg.DB.URL))
	if err != nil {
		return err
	}
	defer m.Close()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil {
			return verr
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q (want up|down|version)", cmd)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("migrate: no change")
		return nil
	}
	if err != nil {
		return err
	}
	slog.Info("migrate: done", "cmd", cmd)
	return nil
}

// trimScheme 將 postgres:// 前綴去掉，golang-migrate 的 pgx5 driver 用自己的 scheme。
func trimScheme(url string) string {
	for _, p := range []string{"postgres://", "postgresql://"} {
		if len(url) > len(p) && url[:len(p)] == p {
			return url[len(p):]
		}
	}
	return url
}
