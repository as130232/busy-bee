package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

func blRouter(limit int64, overrides map[string]int64) (*gin.Engine, *bool) {
	gin.SetMode(gin.TestMode)
	called := false
	e := gin.New()
	e.Use(BodyLimit(limit, overrides))
	h := func(c *gin.Context) {
		called = true
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
			return
		}
		c.Status(http.StatusOK)
	}
	e.POST("/small", h)
	e.POST("/big/:id", h)
	return e, &called
}

func post(e *gin.Engine, path string, n int, chunked bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("a", n)))
	if chunked {
		req.ContentLength = -1
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestBodyLimit_AllowsWithinLimit(t *testing.T) {
	e, _ := blRouter(100, nil)
	if w := post(e, "/small", 100, false); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// Content-Length 已超過上限：不進 handler、直接 413。
func TestBodyLimit_RejectsByContentLength(t *testing.T) {
	e, called := blRouter(100, nil)
	w := post(e, "/small", 101, false)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
	if *called {
		t.Error("handler must not run")
	}
	if !strings.Contains(w.Body.String(), "41302") {
		t.Errorf("body = %s, want errCode 41302", w.Body.String())
	}
}

// 沒有 Content-Length（chunked）：讀到超過上限時中止，不會把整個 body 讀進記憶體。
func TestBodyLimit_RejectsChunkedOverLimit(t *testing.T) {
	e, _ := blRouter(100, nil)
	if w := post(e, "/small", 1000, true); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", w.Code)
	}
}

// 個別路由（以路由樣式比對）可放寬上限。
func TestBodyLimit_RouteOverride(t *testing.T) {
	e, _ := blRouter(100, map[string]int64{"/big/:id": 1000})
	if w := post(e, "/big/1", 1000, false); w.Code != http.StatusOK {
		t.Fatalf("override status = %d, want 200", w.Code)
	}
	if w := post(e, "/big/1", 1001, true); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over override status = %d, want 413", w.Code)
	}
}
