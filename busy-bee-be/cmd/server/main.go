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

	appactionitem "github.com/as130232/busy-bee/busy-bee-be/application/actionitem"
	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	apppush "github.com/as130232/busy-bee/busy-bee-be/application/push"
	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/firebaseauth"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/gcs"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/llm"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/queue"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/stt"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/webpush"
	httpserver "github.com/as130232/busy-bee/busy-bee-be/interface/http"
	actionitemhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/actionitem"
	meetinghandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/meeting"
	pushhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/push"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/ws"
	"github.com/as130232/busy-bee/busy-bee-be/worker"
)

const sweepInterval = 2 * time.Minute

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

	hub := ws.NewHub()
	defer hub.Close()

	llmClient, err := llm.NewGemini(ctx, cfg.Gemini.APIKey, cfg.Gemini.Model)
	if err != nil {
		slog.Error("gemini init failed", "err", err)
		os.Exit(1)
	}
	artifactRepo := db.NewArtifactRepo(pool)
	actionItemRepo := db.NewActionItemRepo(pool)

	// 記憶體佇列（ADR-010）：worker 與 HTTP 同 binary；重啟遺失由 Sweeper 掃 DB 復原
	sttClient := stt.New(cfg.Groq.APIKey)
	processUC := appmeeting.NewProcessUC(appmeeting.ProcessDeps{
		Meetings: meetingRepo, Storage: audioStorage, STT: sttClient,
		Artifacts: artifactRepo, LLM: llmClient, Notifier: hub,
		ActionItems: actionItemRepo, Extractor: llmClient,
	})
	taskQueue := queue.NewMemory(256, queue.DefaultRetryDelays)
	taskQueue.Start(ctx, 2, processUC.Execute, processUC.MarkFailed) // 外部 API bound，低併發

	sweepCtx, stopSweeper := context.WithCancel(ctx)
	defer stopSweeper()
	go worker.NewSweeper(meetingRepo, taskQueue).Run(sweepCtx, sweepInterval)

	// 提醒掃描（F-REMIND，掃描式，ADR-010）：VAPID 未設定則停用
	pushRepo := db.NewPushRepo(pool)
	var pushHandler *pushhandler.Handler
	if cfg.Push.VAPIDPublicKey != "" && cfg.Push.VAPIDPrivateKey != "" {
		sender := webpush.New(cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey, cfg.Push.SubscriberEmail)
		reminderUC := appmeeting.NewReminderUC(meetingRepo, pushRepo, sender)
		go worker.RunReminderSweep(sweepCtx, reminderUC, time.Minute)
		pushHandler = pushhandler.NewHandler(apppush.NewSubscribeUC(pushRepo), cfg.Push.VAPIDPublicKey)
	} else {
		slog.Warn("push reminders disabled: VAPID keys not configured")
	}

	deps := httpserver.Deps{
		Verifier:    verifier,
		UserRepo:    userRepo,
		UserHandler: userhandler.NewHandler(appuser.NewSyncUC(userRepo)),
		MeetingHandler: meetinghandler.NewHandler(meetinghandler.HandlerUCs{
			Create:         appmeeting.NewCreateUC(meetingRepo, audioStorage),
			CompleteUpload: appmeeting.NewCompleteUploadUC(meetingRepo, audioStorage, taskQueue),
			ListArtifacts:  appmeeting.NewListArtifactsUC(meetingRepo, artifactRepo),
			List:           appmeeting.NewListUC(meetingRepo),
			Get:            appmeeting.NewGetUC(meetingRepo),
			Retry:          appmeeting.NewRetryUC(meetingRepo, taskQueue),
			Schedule:       appmeeting.NewScheduleUC(meetingRepo),
		}),
		ActionItemHandler: actionitemhandler.NewHandler(actionitemhandler.HandlerUCs{
			ListByMeeting: appactionitem.NewListByMeetingUC(meetingRepo, actionItemRepo),
			ListPending:   appactionitem.NewListPendingUC(actionItemRepo),
			Toggle:        appactionitem.NewToggleUC(actionItemRepo),
		}),
		PushHandler: pushHandler,
		Hub:         hub,
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

	// 先停掃描與 worker（等進行中任務收尾），再關 HTTP；未完成任務下次啟動由 Sweeper 復原
	stopSweeper()
	taskQueue.Stop(shutdownTimeout)

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
