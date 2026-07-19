package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeSweeper struct {
	calls int
}

func (f *fakeSweeper) SweepOnce(context.Context) { f.calls++ }

func setup(sweeper Sweeper, secret string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := NewHandler(sweeper, secret)
	e.POST("/internal/sweep-reminders", h.SweepReminders)
	return e
}

func TestSweep_ValidKeyTriggersSweep(t *testing.T) {
	sw := &fakeSweeper{}
	e := setup(sw, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/internal/sweep-reminders", nil)
	req.Header.Set("X-Sweep-Key", "s3cret")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if sw.calls != 1 {
		t.Errorf("SweepOnce called %d times, want 1", sw.calls)
	}
}

func TestSweep_WrongKeyRejected(t *testing.T) {
	sw := &fakeSweeper{}
	e := setup(sw, "s3cret")

	req := httptest.NewRequest(http.MethodPost, "/internal/sweep-reminders", nil)
	req.Header.Set("X-Sweep-Key", "wrong")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if sw.calls != 0 {
		t.Errorf("SweepOnce should not run on bad key, got %d calls", sw.calls)
	}
}

func TestSweep_EmptySecretFailsClosed(t *testing.T) {
	sw := &fakeSweeper{}
	e := setup(sw, "") // 未設密鑰 → 端點停用，一律拒絕

	req := httptest.NewRequest(http.MethodPost, "/internal/sweep-reminders", nil)
	req.Header.Set("X-Sweep-Key", "")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (fail closed)", w.Code)
	}
	if sw.calls != 0 {
		t.Errorf("SweepOnce should not run when secret unset, got %d calls", sw.calls)
	}
}
