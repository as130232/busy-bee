package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeUserRepo struct {
	user user.User
	err  error
}

func (f *fakeUserRepo) UpsertByFirebaseUID(_ context.Context, _ user.Identity) (user.User, error) {
	return f.user, f.err
}

func (f *fakeUserRepo) GetByFirebaseUID(_ context.Context, _ string) (user.User, error) {
	return f.user, f.err
}

func TestResolveUser_InjectsDBUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbUser := user.User{ID: uuid.New(), FirebaseUID: "fb-1"}
	var captured uuid.UUID

	e := gin.New()
	e.GET("/p",
		func(c *gin.Context) { // 模擬 Auth 已注入 identity
			c.Request = c.Request.WithContext(user.WithIdentity(c.Request.Context(), user.Identity{UID: "fb-1"}))
		},
		ResolveUser(&fakeUserRepo{user: dbUser}),
		func(c *gin.Context) {
			captured, _ = user.IDFrom(c.Request.Context())
			c.Status(http.StatusOK)
		})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured != dbUser.ID {
		t.Errorf("injected ID = %v, want %v", captured, dbUser.ID)
	}
}

func TestResolveUser_MissingIdentity401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/p", ResolveUser(&fakeUserRepo{}), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestResolveUser_UserNotInDB401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/p",
		func(c *gin.Context) {
			c.Request = c.Request.WithContext(user.WithIdentity(c.Request.Context(), user.Identity{UID: "fb-x"}))
		},
		ResolveUser(&fakeUserRepo{err: user.ErrNotFound}),
		func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/p", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (force re-sync)", w.Code)
	}
}
