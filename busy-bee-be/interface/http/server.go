// Package http 組裝 Gin engine 與 HTTP server（middleware 順序、路由掛載）。
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
	userhandler "github.com/as130232/busy-bee/busy-bee-be/interface/http/handler/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/middleware"
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
	Verifier    domainuser.TokenVerifier
	UserHandler *userhandler.Handler
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
	}

	return e
}

func NewServer(cfg *config.Config, deps Deps) *http.Server {
	return &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: NewEngine(cfg, deps),
	}
}
