package user

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appuser "github.com/as130232/busy-bee/busy-bee-be/application/user"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeRepo struct {
	returnUser domainuser.User
}

func (f *fakeRepo) UpsertByFirebaseUID(_ context.Context, id domainuser.Identity) (domainuser.User, error) {
	u := f.returnUser
	u.FirebaseUID = id.UID
	u.Email = id.Email
	return u, nil
}

func TestSync_ReturnsUserEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(appuser.NewSyncUC(&fakeRepo{returnUser: domainuser.User{ID: uuid.New()}}))

	e := gin.New()
	// 模擬 auth middleware 已注入 identity
	e.POST("/users/sync", func(c *gin.Context) {
		ctx := domainuser.WithIdentity(c.Request.Context(),
			domainuser.Identity{UID: "u1", Email: "a@x.com", Name: "Alice"})
		c.Request = c.Request.WithContext(ctx)
	}, h.Sync)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/users/sync", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	var body struct {
		ErrCode int `json:"errCode"`
		Data    struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Data.Email != "a@x.com" {
		t.Errorf("data.email = %q, want a@x.com", body.Data.Email)
	}
	if body.Data.ID == "" {
		t.Error("data.id missing")
	}
}

func TestSync_MissingIdentity401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(appuser.NewSyncUC(&fakeRepo{}))

	e := gin.New()
	e.POST("/users/sync", h.Sync) // 沒有 identity 注入

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/users/sync", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
