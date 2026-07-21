package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/config"
)

func testEngine(t *testing.T) *TestableEngine {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{Env: "local", Port: "8080"},
		Log:    config.LogConfig{Level: "error"},
	}
	return NewEngine(cfg, Deps{})
}

// TestableEngine 是 *gin.Engine 的別名，避免測試直接 import gin。
// 見 server.go 的型別定義。

func TestHealth(t *testing.T) {
	e := testEngine(t)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		ErrCode int            `json:"errCode"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.ErrCode != 0 {
		t.Errorf("errCode = %d, want 0", body.ErrCode)
	}
	if body.Data["status"] != "ok" {
		t.Errorf("data.status = %v, want ok", body.Data["status"])
	}
}

func TestVersion_ReturnsInjectedBuildInfo(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Env: "local", Port: "8080"},
		Log:    config.LogConfig{Level: "error"},
	}
	e := NewEngine(cfg, Deps{Commit: "a0cbf3a", BuiltAt: "2026-07-21T10:30:00Z"})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/version", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Data["commit"] != "a0cbf3a" {
		t.Errorf("data.commit = %v, want a0cbf3a", body.Data["commit"])
	}
	if body.Data["builtAt"] != "2026-07-21T10:30:00Z" {
		t.Errorf("data.builtAt = %v, want injected build time", body.Data["builtAt"])
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	e := testEngine(t)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	headerID := w.Header().Get("X-Request-Id")
	if headerID == "" {
		t.Fatal("X-Request-Id header missing")
	}
	if !strings.Contains(w.Body.String(), headerID) {
		t.Errorf("traceId in body should equal X-Request-Id header %q; body = %s", headerID, w.Body.String())
	}
}

func TestRecovery_PanicReturnsEnvelopeWithoutLeak(t *testing.T) {
	e := testEngine(t)
	e.GET("/panic", func(c *Context) {
		panic("secret internal detail")
	})

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "secret internal detail") {
		t.Errorf("panic detail leaked to client: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "errCode") {
		t.Errorf("panic response is not the unified envelope: %s", w.Body.String())
	}
}

func TestNotFoundRoute_ReturnsEnvelope(t *testing.T) {
	e := testEngine(t)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/no-such-route", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "errCode") {
		t.Errorf("404 response is not the unified envelope: %s", w.Body.String())
	}
}
