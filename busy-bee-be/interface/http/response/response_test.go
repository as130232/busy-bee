package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/ctxutil"
)

func setupCtx(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) Body {
	t.Helper()
	var b Body
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	return b
}

func TestOK(t *testing.T) {
	c, w := setupCtx(t)

	OK(c, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	b := decode(t, w)
	if b.ErrCode != 0 {
		t.Errorf("errCode = %d, want 0", b.ErrCode)
	}
	if b.Data == nil {
		t.Error("data missing")
	}
}

func TestFail_AppErr(t *testing.T) {
	c, w := setupCtx(t)

	Fail(c, apperr.New(errcode.NotFound))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	b := decode(t, w)
	if b.ErrCode != int(errcode.NotFound) {
		t.Errorf("errCode = %d, want %d", b.ErrCode, errcode.NotFound)
	}
}

func TestFail_WrappedAppErrInChain(t *testing.T) {
	c, w := setupCtx(t)
	inner := apperr.New(errcode.Forbidden)

	Fail(c, errors.Join(errors.New("outer context"), inner))

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (extracted from chain)", w.Code)
	}
}

func TestFail_UnknownErrorMapsToInternalWithoutLeak(t *testing.T) {
	c, w := setupCtx(t)

	Fail(c, errors.New("pq: connection refused at 10.0.0.5"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), "10.0.0.5") {
		t.Errorf("response leaks internal cause: %s", w.Body.String())
	}
	b := decode(t, w)
	if b.ErrCode != int(errcode.Internal) {
		t.Errorf("errCode = %d, want %d", b.ErrCode, errcode.Internal)
	}
}

func TestFail_IncludesTraceID(t *testing.T) {
	c, w := setupCtx(t)
	c.Request = c.Request.WithContext(ctxutil.WithRequestID(c.Request.Context(), "trace-abc"))

	Fail(c, apperr.New(errcode.Param, "title"))

	b := decode(t, w)
	if b.TraceID != "trace-abc" {
		t.Errorf("traceId = %q, want %q", b.TraceID, "trace-abc")
	}
}
