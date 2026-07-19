package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func rlRouter(t *testing.T, rps float64, burst int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(RateLimit(rps, burst))
	e.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })
	return e
}

func hit(e *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.RemoteAddr = ip + ":12345"
	e.ServeHTTP(w, req)
	return w
}

// 閒置 IP 需在下一次請求觸發的惰性清理中被移除（無背景 goroutine，ADR-010 單 instance）。
func TestRateLimit_PrunesIdleEntries(t *testing.T) {
	l := newIPRateLimiter(1, 5, 10*time.Minute)
	current := time.Now()
	l.now = func() time.Time { return current }

	l.allow("1.1.1.1")
	current = current.Add(11 * time.Minute)
	l.allow("2.2.2.2") // 這次請求應順手清掉閒置的 1.1.1.1

	l.mu.Lock()
	_, stale := l.entries["1.1.1.1"]
	_, active := l.entries["2.2.2.2"]
	l.mu.Unlock()
	if stale {
		t.Error("閒置超過 TTL 的 IP 應被清除")
	}
	if !active {
		t.Error("剛活動的 IP 不應被清除")
	}
}

func TestRateLimit_AllowsWithinBurst(t *testing.T) {
	e := rlRouter(t, 1, 5)
	for i := 0; i < 5; i++ {
		if w := hit(e, "10.0.0.1"); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (within burst)", i+1, w.Code)
		}
	}
}

func TestRateLimit_Blocks429AfterBurst(t *testing.T) {
	e := rlRouter(t, 0.001, 2) // 幾乎不補充，burst 2
	hit(e, "10.0.0.2")
	hit(e, "10.0.0.2")

	w := hit(e, "10.0.0.2")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if !strings.Contains(w.Body.String(), "errCode") {
		t.Errorf("429 response should use unified envelope: %s", w.Body.String())
	}
}

func TestRateLimit_PerIPIsolation(t *testing.T) {
	e := rlRouter(t, 0.001, 1)
	hit(e, "10.0.0.3") // 耗盡 10.0.0.3 的額度

	if w := hit(e, "10.0.0.4"); w.Code != http.StatusOK {
		t.Fatalf("other IP status = %d, want 200 (per-IP buckets)", w.Code)
	}
}
