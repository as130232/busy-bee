// Package response 提供統一的 HTTP response envelope。
// 所有 handler 一律經由 OK / Fail 回傳，禁止直接呼叫 c.JSON。
package response

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/ctxutil"
)

type Body struct {
	ErrCode int    `json:"errCode"`
	Msg     string `json:"msg"`
	Data    any    `json:"data,omitempty"`
	TraceID string `json:"traceId,omitempty"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{
		ErrCode: int(errcode.Success),
		Msg:     errcode.FormatMsg(errcode.Success),
		Data:    data,
		TraceID: ctxutil.RequestID(c.Request.Context()),
	})
}

// Fail 從 err chain 取出 *apperr.Error 映射 HTTP status；
// 非 apperr 錯誤一律視為 Internal，原始錯誤只進 log。
func Fail(c *gin.Context, err error) {
	ctx := c.Request.Context()

	var ae *apperr.Error
	if !errors.As(err, &ae) {
		slog.ErrorContext(ctx, "unhandled error", "err", err, "path", c.Request.URL.Path)
		ae = apperr.New(errcode.Internal)
	} else if ae.Cause != nil {
		slog.ErrorContext(ctx, "request failed", "err", ae, "path", c.Request.URL.Path)
	}

	c.AbortWithStatusJSON(errcode.HTTPStatus(ae.Code), Body{
		ErrCode: int(ae.Code),
		Msg:     ae.ClientMsg(),
		TraceID: ctxutil.RequestID(ctx),
	})
}
