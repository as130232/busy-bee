// Package http 組裝 Gin engine 與 HTTP server（middleware 順序、路由掛載）。
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
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

// NewEngine 組裝 middleware 鏈與路由。順序：Recovery 最外層 → RequestID → Logger。
func NewEngine(cfg *config.Config) *gin.Engine {
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

	return e
}

func NewServer(cfg *config.Config) *http.Server {
	return &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: NewEngine(cfg),
	}
}
