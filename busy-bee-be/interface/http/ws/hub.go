// Package ws 提供單機 in-process WebSocket hub（ADR-002 修訂版、ADR-010）。
// 瀏覽器 WS 無法帶 Authorization header：連線後第一則訊息帶 Firebase JWT，
// 驗證（含 email 白名單）通過前不綁定 user、不推送任何資料。
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

const (
	authTimeout  = 10 * time.Second
	writeTimeout = 5 * time.Second
	sendBuffer   = 16
)

type client struct {
	userID uuid.UUID
	send   chan []byte
}

type Hub struct {
	mu      sync.Mutex
	clients map[uuid.UUID]map[*client]struct{}
	closed  chan struct{}
}

var _ domainmeeting.StatusNotifier = (*Hub)(nil)

func NewHub() *Hub {
	return &Hub{
		clients: make(map[uuid.UUID]map[*client]struct{}),
		closed:  make(chan struct{}),
	}
}

// Close 停止 hub（釋放全部連線的 send channel 監聽）。
func (h *Hub) Close() {
	close(h.closed)
}

// NotifyStatus 實作 domain/meeting.StatusNotifier：推給該 user 的所有連線。
// send buffer 滿（慢連線）直接丟棄該則——前端重連後會補拉最新狀態。
func (h *Hub) NotifyStatus(ctx context.Context, e domainmeeting.StatusEvent) {
	payload, err := json.Marshal(gin.H{
		"type":         "meetingStatus",
		"meetingId":    e.MeetingID.String(),
		"status":       string(e.Status),
		"errorMessage": e.ErrorMessage,
	})
	if err != nil {
		slog.ErrorContext(ctx, "ws.notify.marshal", "err", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[e.UserID] {
		select {
		case c.send <- payload:
		default:
			slog.WarnContext(ctx, "ws.notify.dropped", "user_id", e.UserID)
		}
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[c.userID] == nil {
		h.clients[c.userID] = make(map[*client]struct{})
	}
	h.clients[c.userID][c] = struct{}{}
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[c.userID], c)
	if len(h.clients[c.userID]) == 0 {
		delete(h.clients, c.userID)
	}
}

type authMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// Handler GET /api/v1/ws — 不掛 Auth middleware（瀏覽器 WS 帶不了 header），
// 改由第一則訊息驗證。未用 cookie 認證，跨源連線無 ambient credentials 風險。
func (h *Hub) Handler(verifier domainuser.TokenVerifier, userRepo domainuser.Repository, allowedEmails []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedEmails))
	for _, e := range allowedEmails {
		allowed[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
	}

	return func(c *gin.Context) {
		conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // 見上：認證靠 token，非 cookie
		})
		if err != nil {
			return
		}
		defer conn.CloseNow()

		ctx := c.Request.Context()

		userID, err := h.authenticate(ctx, conn, verifier, userRepo, allowed)
		if err != nil {
			slog.InfoContext(ctx, "ws.auth.rejected", "err", err)
			conn.Close(websocket.StatusPolicyViolation, "unauthorized")
			return
		}

		cl := &client{userID: userID, send: make(chan []byte, sendBuffer)}
		h.register(cl)
		defer h.unregister(cl)

		if err := h.write(ctx, conn, []byte(`{"type":"authOk"}`)); err != nil {
			return
		}

		// reader：只為偵測關閉（忽略後續訊息）；關閉時取消 ctx 結束 writer
		readCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			defer cancel()
			for {
				if _, _, err := conn.Read(readCtx); err != nil {
					return
				}
			}
		}()

		// writer：推送事件直到連線關閉或 hub 結束
		for {
			select {
			case <-readCtx.Done():
				return
			case <-h.closed:
				return
			case payload := <-cl.send:
				if err := h.write(readCtx, conn, payload); err != nil {
					return
				}
			}
		}
	}
}

func (h *Hub) authenticate(ctx context.Context, conn *websocket.Conn, verifier domainuser.TokenVerifier, userRepo domainuser.Repository, allowed map[string]struct{}) (uuid.UUID, error) {
	authCtx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	_, data, err := conn.Read(authCtx)
	if err != nil {
		return uuid.Nil, err
	}
	var msg authMessage
	if err := json.Unmarshal(data, &msg); err != nil || msg.Type != "auth" || msg.Token == "" {
		return uuid.Nil, errInvalidAuthMessage
	}

	identity, err := verifier.Verify(authCtx, msg.Token)
	if err != nil {
		return uuid.Nil, err
	}
	if _, ok := allowed[strings.ToLower(identity.Email)]; !ok {
		return uuid.Nil, errNotWhitelisted
	}

	u, err := userRepo.GetByFirebaseUID(authCtx, identity.UID)
	if err != nil {
		return uuid.Nil, err
	}
	return u.ID, nil
}

func (h *Hub) write(ctx context.Context, conn *websocket.Conn, payload []byte) error {
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, payload)
}

var (
	errInvalidAuthMessage = jsonError("invalid auth message")
	errNotWhitelisted     = jsonError("email not whitelisted")
)

type jsonError string

func (e jsonError) Error() string { return string(e) }
