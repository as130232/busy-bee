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

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/firebaseauth"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/gcs"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/queue"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/stt"
	httpserver "github.com/as130232/busy-bee/busy-bee-be/interface/http"
	meetinghandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/meeting"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/worker"

	"github.com/hibiken/asynq"
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

	audioStorage, err := gcs.New(ctx, cfg.GCS.Bucket, cfg.GCS.SignerEmail)
	if err != nil {
		slog.Error("gcs init failed", "err", err)
		os.Exit(1)
	}
	defer audioStorage.Close()

	userRepo := db.NewUserRepo(pool)
	meetingRepo := db.NewMeetingRepo(pool)

	taskQueue := queue.NewAsynq(cfg.Redis.Addr, cfg.Redis.Password, 0)
	defer taskQueue.Close()

	// Asynq worker 與 HTTP server 同 binary（ADR-004）
	sttClient := stt.New(cfg.Groq.APIKey)
	processUC := appmeeting.NewProcessUC(meetingRepo, audioStorage, sttClient)
	asynqSrv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password},
		asynq.Config{Concurrency: 2}, // STT/LLM 皆為外部 API bound，低併發即可
	)
	go func() {
		if err := asynqSrv.Run(worker.NewMux(processUC)); err != nil {
			slog.Error("asynq server failed", "err", err)
			os.Exit(1)
		}
	}()

	deps := httpserver.Deps{
		Verifier:    verifier,
		UserRepo:    userRepo,
		UserHandler: userhandler.NewHandler(appuser.NewSyncUC(userRepo)),
		MeetingHandler: meetinghandler.NewHandler(
			appmeeting.NewCreateUC(meetingRepo, audioStorage),
			appmeeting.NewCompleteUploadUC(meetingRepo, audioStorage, taskQueue),
		),
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

	// 先停 worker（等待進行中任務結束或歸還佇列），再關 HTTP
	asynqSrv.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
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
