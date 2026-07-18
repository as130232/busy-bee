// Package http 組裝 Gin engine 與 HTTP server（middleware 順序、路由掛載）。
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	meetinghandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/meeting"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/middleware"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/ws"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
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
	Verifier       domainuser.TokenVerifier
	UserRepo       domainuser.Repository
	UserHandler    *userhandler.Handler
	MeetingHandler *meetinghandler.Handler
	Hub            *ws.Hub
}

// NewEngine 組裝 middleware 鏈與路由。順序：Recovery 最外層 → RequestID → Logger。
func NewEngine(cfg *config.Config, deps Deps) *gin.Engine {
	if cfg.Server.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}
	e := gin.New()
	e.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.RequestLogger(),
	)

	e.NoRoute(func(c *gin.Context) {
		response.Fail(c, apperr.New(errcode.NotFound))
	})

	e.GET("/health", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok", "env": cfg.Server.Env})
	})

	// 受保護路由：需通過 Firebase JWT 驗證 + email 白名單
	if deps.Verifier != nil {
		v1 := e.Group("/api/v1", middleware.Auth(deps.Verifier, cfg.Auth.AllowedEmails))
		v1.POST("/users/sync", deps.UserHandler.Sync)

		// 需要 DB 用戶身分的路由再掛 ResolveUser
		authed := v1.Group("", middleware.ResolveUser(deps.UserRepo))
		authed.POST("/meetings", deps.MeetingHandler.Create)
		authed.GET("/meetings", deps.MeetingHandler.List)
		authed.GET("/meetings/:id", deps.MeetingHandler.Get)
		authed.POST("/meetings/:id/complete-upload", deps.MeetingHandler.CompleteUpload)
		authed.POST("/meetings/:id/retry", deps.MeetingHandler.Retry)
		authed.GET("/meetings/:id/artifacts", deps.MeetingHandler.ListArtifacts)
	}

	// WS 不掛 Auth middleware（瀏覽器帶不了 header），第一則訊息驗證（ADR-002）
	if deps.Hub != nil {
		e.GET("/api/v1/ws", deps.Hub.Handler(deps.Verifier, deps.UserRepo, cfg.Auth.AllowedEmails))
	}

	return e
}

func NewServer(cfg *config.Config, deps Deps) *http.Server {
	return &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: NewEngine(cfg, deps),
	}
}
