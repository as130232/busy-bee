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
	// LiveMicTranscription 即時會議是否同時即時辨識麥克風（F-LIVE 設定開關；預設關＝省錢版）。
	LiveMicTranscription bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Identity 通過身份驗證後的請求者（來自 ID token claims）。
type Identity struct {
	UID     string
	Email   string
	Name    string
	Picture string
	// EmailVerified token 的 email_verified claim；白名單只認已驗證的 email。
	EmailVerified bool
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

// PreferenceWriter 更新使用者偏好。
type PreferenceWriter interface {
	SetLiveMicTranscription(ctx context.Context, userID uuid.UUID, enabled bool) (User, error)
}
