// Package push 定義 Web Push 訂閱 entity 與 port interface（零外部依賴）。
package push

import (
	"context"

	"github.com/google/uuid"
)

// Subscription 一個瀏覽器端點的推播訂閱。
type Subscription struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	Endpoint string
	P256dh   string
	Auth     string
}

// Repository 訂閱存取 port（pgx 實作在 infrastructure/db）。
type Repository interface {
	// Upsert 以 endpoint 唯一；換帳號登入時訂閱歸屬轉移。
	Upsert(ctx context.Context, sub Subscription) (Subscription, error)
	DeleteByEndpoint(ctx context.Context, endpoint string) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Subscription, error)
}

// Message 推播內容。
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

// ErrSubscriptionGone 端點已失效（瀏覽器取消訂閱），caller 應刪除該訂閱。
type ErrSubscriptionGone struct{ Endpoint string }

func (e ErrSubscriptionGone) Error() string { return "push subscription gone: " + e.Endpoint }

// Sender Web Push 發送 port（webpush 實作在 infrastructure/webpush）。
type Sender interface {
	Send(ctx context.Context, sub Subscription, msg Message) error
}
