package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/firebaseauth"
	httpserver "github.com/as130232/busy-bee/busy-bee-be/interface/http"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
)

const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	setupLogger(cfg)

	ctx := context.Background()

	pool, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	verifier, err := firebaseauth.New(ctx, cfg.Auth.FirebaseProjectID)
	if err != nil {
		slog.Error("firebase auth init failed", "err", err)
		os.Exit(1)
	}

	deps := httpserver.Deps{
		Verifier:    verifier,
		UserHandler: userhandler.NewHandler(appuser.NewSyncUC(db.NewUserRepo(pool))),
	}
	srv := httpserver.NewServer(cfg, deps)

	go func() {
		slog.Info("server starting", "addr", srv.Addr, "env", cfg.Server.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down", "timeout", shutdownTimeout.String())
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func setupLogger(cfg *config.Config) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
