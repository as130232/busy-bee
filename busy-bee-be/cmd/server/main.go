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
	appcorrection "github.com/as130232/busy-bee/busy-bee-be/application/correction"
	applive "github.com/as130232/busy-bee/busy-bee-be/application/live"
	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	appmc "github.com/as130232/busy-bee/busy-bee-be/application/meetingcontext"
	apppush "github.com/as130232/busy-bee/busy-bee-be/application/push"
	appsearch "github.com/as130232/busy-bee/busy-bee-be/application/search"
	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/fetch"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/firebaseauth"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/gcs"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/llm"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/queue"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/stt"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/tempslot"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/webpush"
	httpserver "github.com/as130232/busy-bee/busy-bee-be/interface/http"
	actionitemhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/actionitem"
	correctionhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/correction"
	livehandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/live"
	meetinghandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/meeting"
	opshandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/ops"
	pushhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/push"
	qahandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/qa"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/ws"
	"github.com/as130232/busy-bee/busy-bee-be/worker"
)

const sweepInterval = 2 * time.Minute
const indexBackfillInterval = 5 * time.Minute

// 建置資訊，由 Dockerfile 經 -ldflags "-X main.commit=... -X main.builtAt=..." 注入；
// 本地未注入時為預設值。/version 端點回傳，供部署後確認上線的 commit。
var (
	commit  = "dev"
	builtAt = "unknown"
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

	hub := ws.NewHub()
	defer hub.Close()

	llmClient, err := llm.NewGemini(ctx, cfg.Gemini.APIKey, cfg.Gemini.Model, llm.WithFallbackModel(cfg.Gemini.FallbackModel))
	if err != nil {
		slog.Error("gemini init failed", "err", err)
		os.Exit(1)
	}
	artifactRepo := db.NewArtifactRepo(pool)
	actionItemRepo := db.NewActionItemRepo(pool)

	// 語意搜尋（Phase 15）：chunk_repo + IndexUC；completed 後 best-effort 觸發，未索引由回填掃描補
	chunkRepo := db.NewChunkRepo(pool)
	indexUC := appsearch.NewIndexUC(meetingRepo, llmClient, chunkRepo)

	// 記憶體佇列（ADR-010）：worker 與 HTTP 同 binary；重啟遺失由 Sweeper 掃 DB 復原
	// STT 用 Deepgram（聲學語者分離，一個聲音＝一位講者，較 LLM 推測式穩定）。
	sttClient := stt.NewDeepgram(cfg.Deepgram.APIKey, cfg.Deepgram.Model, cfg.Deepgram.Language, cfg.Deepgram.Keywords)
	// 大檔暫存名額：/tmp 在 Cloud Run 上算記憶體（512Mi），貼連結下載與 AI 校正壓縮共用，同時只落地一個大檔
	bigTempSlots := tempslot.New(1)
	// 匯入來源音訊抓取（貼連結：直接音檔/Podcast 走 HTTP，YouTube 走 yt-dlp）
	audioFetcher := fetch.New(fetch.WithTempSlots(bigTempSlots))
	// 會前資料（F-LIVE）：處理管線、AI 校正與即時會議共用
	contextRepo := db.NewMeetingContextRepo(pool)
	processUC := appmeeting.NewProcessUC(appmeeting.ProcessDeps{
		Meetings: meetingRepo, Storage: audioStorage, STT: sttClient,
		Artifacts: artifactRepo, Summarizer: llmClient, Notifier: hub,
		Extractor: llmClient, Saver: db.NewProcessRepo(pool),
		Indexer: indexUC, Fetcher: audioFetcher, Renamer: meetingRepo, TagWriter: meetingRepo,
		Contexts: contextRepo,
	})
	taskQueue := queue.NewMemory(256, cfg.Worker.TaskTimeout, queue.DefaultRetryDelays)
	taskQueue.Start(ctx, 2, processUC.Execute, processUC.MarkFailed) // 外部 API bound，低併發

	sweepCtx, stopSweeper := context.WithCancel(ctx)
	defer stopSweeper()
	go worker.NewSweeper(meetingRepo, taskQueue).Run(sweepCtx, sweepInterval)
	go worker.RunIndexBackfill(sweepCtx, chunkRepo, indexUC, indexBackfillInterval)

	// AI 錄音重新校正（Phase 27, F-CORRECT）：獨立佇列（外部 API bound，併發 1）；重啟由 correction sweeper 復原
	correctionRepo := db.NewCorrectionRepo(pool)
	executeCorrection := appcorrection.NewExecuteUC(appcorrection.ExecuteDeps{
		Repo: correctionRepo, Meetings: meetingRepo, Storage: audioStorage,
		Clipper: stt.FFmpegClipper{}, Corrector: llmClient, Summarizer: llmClient,
		Extractor: llmClient, Items: actionItemRepo, Indexer: indexUC, Notifier: hub,
		Contexts: contextRepo, Compress: bigTempSlots.GuardTempFile(stt.CompressToTempFile),
	})
	correctionQueue := queue.NewMemory(64, cfg.Worker.TaskTimeout, queue.DefaultRetryDelays)
	correctionQueue.Start(ctx, 1, executeCorrection.Execute, executeCorrection.MarkFailed)
	go worker.NewSweeperFunc(correctionRepo.ListUnfinishedIDs, correctionQueue.EnqueueCorrection).Run(sweepCtx, sweepInterval)

	// 提醒掃描（F-REMIND）：VAPID 未設定則停用。
	// scale-to-zero 下 instance 常態為 0，進程內 ticker 只在 instance 存活時觸發（ADR-004）；
	// 成本考量未啟用 Cloud Scheduler（2026-10-13 決策），提醒會延到 instance 被喚醒後才補發。
	// /internal/sweep-reminders 端點保留，日後要準時提醒時由外部排程呼叫即可（與 ticker 冪等，reminded_at 防重複）。
	pushRepo := db.NewPushRepo(pool)
	var pushHandler *pushhandler.Handler
	var reminderUC *appmeeting.ReminderUC
	if cfg.Push.VAPIDPublicKey != "" && cfg.Push.VAPIDPrivateKey != "" {
		sender := webpush.New(cfg.Push.VAPIDPublicKey, cfg.Push.VAPIDPrivateKey, cfg.Push.SubscriberEmail)
		reminderUC = appmeeting.NewReminderUC(meetingRepo, pushRepo, sender, actionItemRepo)
		go worker.RunReminderSweep(sweepCtx, reminderUC, time.Minute)
		pushHandler = pushhandler.NewHandler(apppush.NewSubscribeUC(pushRepo), apppush.NewTestUC(pushRepo, sender), cfg.Push.VAPIDPublicKey)
	} else {
		slog.Warn("push reminders disabled: VAPID keys not configured")
	}

	// 即時會議（Phase 30, F-LIVE）：瀏覽器以短效金鑰直連 Deepgram 串流，後端只負責 Gemini、用量與狀態（ADR-018）
	liveUC := applive.New(applive.Deps{
		Meetings: meetingRepo, Repo: meetingRepo, Contexts: contextRepo, Storage: audioStorage,
		Tokens: stt.NewDeepgramGrant(cfg.Deepgram.APIKey), Assistant: llmClient,
	}, applive.Config{
		MonthlyMinutesLimit: cfg.Live.MonthlyMinutesLimit, STTModel: cfg.Live.STTModel, Keyterms: cfg.Deepgram.Keywords,
		MaxNotesChars: 20000, MaxAskChars: 120000,
	})
	contextUC := appmc.New(meetingRepo, contextRepo, llmClient, appmc.Config{
		MaxChars: int64(cfg.Live.MaxContextChars), MaxFileBytes: int64(cfg.Live.MaxContextFileBytes),
	})

	var internalHandler *opshandler.Handler
	if reminderUC != nil {
		internalHandler = opshandler.NewHandler(reminderUC, cfg.Push.SweepSecret)
	}

	deps := httpserver.Deps{
		Verifier:    verifier,
		UserRepo:    userRepo,
		UserHandler: userhandler.NewHandler(appuser.NewSyncUC(userRepo)),
		CorrectionHandler: correctionhandler.NewHandler(
			appcorrection.NewGetUC(meetingRepo, correctionRepo),
			appcorrection.NewStartUC(meetingRepo, correctionRepo, correctionQueue),
			appcorrection.NewRevertUC(correctionRepo, indexUC),
		),
		MeetingHandler: meetinghandler.NewHandler(meetinghandler.HandlerUCs{
			Create:         appmeeting.NewCreateUC(meetingRepo, audioStorage),
			Import:         appmeeting.NewImportUC(meetingRepo, taskQueue),
			CompleteUpload: appmeeting.NewCompleteUploadUC(meetingRepo, audioStorage, taskQueue),
			ListArtifacts:  appmeeting.NewListArtifactsUC(meetingRepo, artifactRepo),
			List:           appmeeting.NewListUC(meetingRepo),
			Get:            appmeeting.NewGetUC(meetingRepo),
			AudioURL:       appmeeting.NewAudioURLUC(meetingRepo, audioStorage),
			Retry:          appmeeting.NewRetryUC(meetingRepo, taskQueue),
			Schedule:       appmeeting.NewScheduleUC(meetingRepo),
			Manage:         appmeeting.NewManageUC(meetingRepo, audioStorage),
			EditSegment:    appmeeting.NewEditSegmentUC(meetingRepo, indexUC),
			Search:         appsearch.NewSearchUC(meetingRepo, llmClient, chunkRepo, meetingRepo),
		}),
		LiveHandler: livehandler.NewHandler(liveUC, contextUC, userRepo, int64(cfg.Live.MaxContextFileBytes)),
		QAHandler:   qahandler.NewHandler(appsearch.NewQAUC(llmClient, chunkRepo, llmClient, meetingRepo)),
		ActionItemHandler: actionitemhandler.NewHandler(actionitemhandler.HandlerUCs{
			ListByMeeting: appactionitem.NewListByMeetingUC(meetingRepo, actionItemRepo),
			ListPending:   appactionitem.NewListPendingUC(actionItemRepo),
			Toggle:        appactionitem.NewToggleUC(actionItemRepo),
			Add:           appactionitem.NewAddUC(meetingRepo, actionItemRepo),
			Edit:          appactionitem.NewEditUC(actionItemRepo),
			Delete:        appactionitem.NewDeleteUC(actionItemRepo),
		}),
		PushHandler:     pushHandler,
		InternalHandler: internalHandler,
		Hub:             hub,
		Commit:          commit,
		BuiltAt:         builtAt,
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
	correctionQueue.Stop(shutdownTimeout)

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
