// Package user 定義用戶 entity 與相關 port interface（零外部依賴）。
package user

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound 用戶不存在（尚未 /users/sync）。
var ErrNotFound = errors.New("user not found")

// User 資料庫中的用戶。
type User struct {
	ID          uuid.UUID
	FirebaseUID string
	Email       string
	DisplayName string
	AvatarURL   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Identity 通過身份驗證後的請求者（來自 ID token claims）。
type Identity struct {
	UID     string
	Email   string
	Name    string
	Picture string
}

// TokenVerifier 驗證 ID token（Firebase 實作在 infrastructure/firebaseauth）。
type TokenVerifier interface {
	Verify(ctx context.Context, idToken string) (Identity, error)
}

// Repository 用戶資料存取 port（pgx 實作在 infrastructure/db）。
type Repository interface {
	UpsertByFirebaseUID(ctx context.Context, identity Identity) (User, error)
	GetByFirebaseUID(ctx context.Context, firebaseUID string) (User, error)
}
