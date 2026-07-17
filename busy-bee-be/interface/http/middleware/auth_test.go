package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeVerifier struct {
	identity user.Identity
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (user.Identity, error) {
	return f.identity, f.err
}

func runAuth(t *testing.T, v user.TokenVerifier, allowed []string, authHeader string) (*httptest.ResponseRecorder, *user.Identity) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var captured *user.Identity

	e := gin.New()
	e.GET("/p", Auth(v, allowed), func(c *gin.Context) {
		if id, ok := user.IdentityFrom(c.Request.Context()); ok {
			captured = &id
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	e.ServeHTTP(w, req)
	return w, captured
}

func TestAuth_MissingHeader401(t *testing.T) {
	w, _ := runAuth(t, fakeVerifier{}, []string{"a@x.com"}, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuth_MalformedHeader401(t *testing.T) {
	w, _ := runAuth(t, fakeVerifier{}, []string{"a@x.com"}, "NotBearer xyz")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuth_VerifierError401(t *testing.T) {
	v := fakeVerifier{err: errors.New("token expired")}
	w, _ := runAuth(t, v, []string{"a@x.com"}, "Bearer tok")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuth_EmailNotWhitelisted403(t *testing.T) {
	v := fakeVerifier{identity: user.Identity{UID: "u1", Email: "stranger@x.com"}}
	w, _ := runAuth(t, v, []string{"a@x.com", "b@x.com"}, "Bearer tok")
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAuth_EmptyWhitelistFailClosed403(t *testing.T) {
	v := fakeVerifier{identity: user.Identity{UID: "u1", Email: "a@x.com"}}
	w, _ := runAuth(t, v, nil, "Bearer tok")
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (fail closed)", w.Code)
	}
}

func TestAuth_WhitelistCaseInsensitive(t *testing.T) {
	v := fakeVerifier{identity: user.Identity{UID: "u1", Email: "A@X.com"}}
	w, _ := runAuth(t, v, []string{"a@x.com"}, "Bearer tok")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (email compare case-insensitive)", w.Code)
	}
}

func TestAuth_ValidTokenInjectsIdentity(t *testing.T) {
	id := user.Identity{UID: "u1", Email: "a@x.com", Name: "Alice"}
	w, captured := runAuth(t, fakeVerifier{identity: id}, []string{"a@x.com"}, "Bearer tok")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured == nil {
		t.Fatal("identity not injected into request context")
	}
	if *captured != id {
		t.Errorf("identity = %+v, want %+v", *captured, id)
	}
}
