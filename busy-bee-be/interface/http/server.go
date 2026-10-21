// Package http 組裝 Gin engine 與 HTTP server（middleware 順序、路由掛載）。
package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	actionitemhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/actionitem"
	correctionhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/correction"
	livehandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/live"
	meetinghandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/meeting"
	opshandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/ops"
	pushhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/push"
	qahandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/qa"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/middleware"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/ws"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// TestableEngine / Context 別名讓測試不直接 import gin。
type (
	TestableEngine = gin.Engine
	Context        = gin.Context
)

// Deps 路由所需的依賴，由 main（或測試）組裝注入。
type Deps struct {
	Verifier          domainuser.TokenVerifier
	UserRepo          domainuser.Repository
	UserHandler       *userhandler.Handler
	MeetingHandler    *meetinghandler.Handler
	ActionItemHandler *actionitemhandler.Handler
	// CorrectionHandler AI 錄音重新校正（F-CORRECT）；nil 時不掛路由。
	CorrectionHandler *correctionhandler.Handler
	// LiveHandler 即時會議與會前資料（F-LIVE）；nil 時不掛路由。
	LiveHandler     *livehandler.Handler
	QAHandler       *qahandler.Handler
	PushHandler     *pushhandler.Handler
	InternalHandler *opshandler.Handler
	Hub             *ws.Hub
	// 建置資訊（由 main 經 ldflags 注入）；/version 回傳，供部署後確認上線的 commit。
	Commit  string
	BuiltAt string
}

// defaultBodyLimit 一般 API 請求 body 上限；音檔走 GCS 直傳，不經 API。
const defaultBodyLimit = 1 << 20

// NewEngine 組裝 middleware 鏈與路由。順序：Recovery 最外層 → RequestID → Logger。
func NewEngine(cfg *config.Config, deps Deps) *gin.Engine {
	if cfg.Server.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.New()
	e.Use(
		middleware.Recovery(),
		middleware.SecurityHeaders(cfg.Server.Env == "prod"),
		middleware.RequestID(),
		middleware.RequestLogger(),
		// 預設 1MB（即時問答帶整場逐字稿約 12 萬字 ≈ 360KB）；會前資料上傳另依檔案上限放寬
		middleware.BodyLimit(defaultBodyLimit, map[string]int64{
			"/api/v1/meetings/:id/contexts": int64(cfg.Live.MaxContextFileBytes) + 64<<10,
		}),
	)

	e.NoRoute(func(c *gin.Context) {
		response.Fail(c, apperr.New(errcode.NotFound))
	})

	e.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok", "env": cfg.Server.Env})
	})

	// 建置資訊（公開、免 auth）：部署後 curl /version 確認線上跑的 commit。
	e.GET("/version", func(c *gin.Context) {
		response.OK(c, gin.H{"commit": deps.Commit, "builtAt": deps.BuiltAt, "env": cfg.Server.Env})
	})

	// 內部維運端點（不經 Firebase auth，改以共享密鑰保護）：Cloud Scheduler 觸發提醒掃描
	if deps.InternalHandler != nil {
		e.POST("/internal/sweep-reminders", deps.InternalHandler.SweepReminders)
	}

	// 受保護路由：Firebase JWT 驗證 + email 白名單 → 每位使用者限流（以 UID 為 key，XFF 無法繞過）
	if deps.Verifier != nil {
		v1 := e.Group("/api/v1",
			middleware.Auth(deps.Verifier, cfg.Auth.AllowedEmails),
			middleware.RateLimit(10, 30)) // 每位使用者 10 req/s、burst 30
		v1.POST("/users/sync", deps.UserHandler.Sync)

		// 需要 DB 用戶身分的路由再掛 ResolveUser
		authed := v1.Group("", middleware.ResolveUser(deps.UserRepo))
		authed.POST("/meetings", deps.MeetingHandler.Create)
		authed.POST("/meetings/import", deps.MeetingHandler.Import)
		authed.GET("/meetings", deps.MeetingHandler.List)
		authed.POST("/meetings/scheduled", deps.MeetingHandler.CreateScheduled)
		authed.GET("/meetings/:id", deps.MeetingHandler.Get)
		authed.GET("/meetings/:id/audio-url", deps.MeetingHandler.AudioURL)
		authed.PATCH("/meetings/:id", deps.MeetingHandler.Rename)
		authed.PATCH("/meetings/:id/speakers", deps.MeetingHandler.UpdateSpeakers)
		authed.PATCH("/meetings/:id/tags", deps.MeetingHandler.UpdateTags)
		authed.PATCH("/meetings/:id/transcript", deps.MeetingHandler.EditSegment)
		authed.POST("/meetings/:id/transcript/replace", deps.MeetingHandler.ReplaceText)
		authed.DELETE("/meetings/:id", deps.MeetingHandler.Delete)
		authed.POST("/meetings/:id/complete-upload", deps.MeetingHandler.CompleteUpload)
		authed.PATCH("/meetings/:id/schedule", deps.MeetingHandler.UpdateSchedule)
		authed.POST("/meetings/:id/retry", deps.MeetingHandler.Retry)
		if deps.CorrectionHandler != nil {
			authed.GET("/meetings/:id/correction", deps.CorrectionHandler.Get)
			authed.POST("/meetings/:id/correction", deps.CorrectionHandler.Start)
			authed.POST("/meetings/:id/correction/revert", deps.CorrectionHandler.Revert)
		}
		authed.GET("/meetings/:id/artifacts", deps.MeetingHandler.ListArtifacts)
		if deps.LiveHandler != nil {
			deps.LiveHandler.Register(authed)
		}

		if deps.ActionItemHandler != nil {
			authed.GET("/meetings/:id/action-items", deps.ActionItemHandler.ListByMeeting)
			authed.POST("/meetings/:id/action-items", deps.ActionItemHandler.Add)
			authed.GET("/action-items", deps.ActionItemHandler.ListPending)
			authed.PATCH("/action-items/:id", deps.ActionItemHandler.Update)
			authed.DELETE("/action-items/:id", deps.ActionItemHandler.Delete)
		}

		if deps.QAHandler != nil {
			authed.POST("/meetings/qa", deps.QAHandler.Ask)
		}

		if deps.PushHandler != nil {
			authed.GET("/push/vapid-public-key", deps.PushHandler.VAPIDPublicKey)
			authed.POST("/push/subscriptions", deps.PushHandler.Subscribe)
			authed.DELETE("/push/subscriptions", deps.PushHandler.Unsubscribe)
			authed.POST("/push/test", deps.PushHandler.Test) // debug / demo：立即送測試推播驗證顯示層
		}
	}

	// WS 不掛 Auth middleware（瀏覽器帶不了 header），第一則訊息驗證（ADR-002）
	if deps.Hub != nil {
		e.GET("/api/v1/ws", deps.Hub.Handler(deps.Verifier, deps.UserRepo, cfg.Auth.AllowedEmails, cfg.Server.AllowedOrigins))
	}

	return e
}

func NewServer(cfg *config.Config, deps Deps) *http.Server {
	return &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: NewEngine(cfg, deps),
		// 擋慢速送 header 占住連線；不設 ReadTimeout / WriteTimeout（WebSocket 長連線、即時問答較慢），
		// body 大小由 BodyLimit 限制，整體請求時間由 Cloud Run 300 秒上限兜底。
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}
