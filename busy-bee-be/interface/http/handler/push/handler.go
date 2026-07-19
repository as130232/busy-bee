// Package push 提供推播訂閱 HTTP handlers。
package push

import (
	"github.com/gin-gonic/gin"

	apppush "github.com/as130232/busy-bee/busy-bee-be/application/push"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type Handler struct {
	uc             *apppush.SubscribeUC
	vapidPublicKey string
}

func NewHandler(uc *apppush.SubscribeUC, vapidPublicKey string) *Handler {
	return &Handler{uc: uc, vapidPublicKey: vapidPublicKey}
}

// VAPIDPublicKey GET /api/v1/push/vapid-public-key — 前端訂閱所需的公鑰。
func (h *Handler) VAPIDPublicKey(c *gin.Context) {
	response.OK(c, gin.H{"publicKey": h.vapidPublicKey})
}

type subscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Subscribe POST /api/v1/push/subscriptions
func (h *Handler) Subscribe(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	var req subscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	if err := h.uc.Subscribe(c.Request.Context(), userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"subscribed": true})
}

type unsubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// Unsubscribe DELETE /api/v1/push/subscriptions
func (h *Handler) Unsubscribe(c *gin.Context) {
	var req unsubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}
	if err := h.uc.Unsubscribe(c.Request.Context(), req.Endpoint); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"subscribed": false})
}
