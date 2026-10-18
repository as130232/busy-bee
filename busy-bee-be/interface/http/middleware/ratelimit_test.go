package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
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

// 模擬 Auth 之後：context 帶 Identity，限流改以 UID 為 key。
func userRouter(rps float64, burst int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(func(c *gin.Context) {
		if uid := c.GetHeader("X-Test-UID"); uid != "" {
			c.Request = c.Request.WithContext(user.WithIdentity(c.Request.Context(), user.Identity{UID: uid}))
		}
		c.Next()
	}, RateLimit(rps, burst))
	e.GET("/p", func(c *gin.Context) { c.Status(http.StatusOK) })
	return e
}

func hitAs(e *gin.Engine, uid, xff string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.RemoteAddr = "169.254.1.1:12345"
	req.Header.Set("X-Test-UID", uid)
	req.Header.Set("X-Forwarded-For", xff)
	e.ServeHTTP(w, req)
	return w
}

// 同一使用者每次換 X-Forwarded-For 也不能換到新額度（舊版以 ClientIP 為 key 可被繞過）。
func TestRateLimit_SpoofedXFFDoesNotBypassUserLimit(t *testing.T) {
	e := userRouter(0.001, 2)
	hitAs(e, "uid-a", "1.1.1.1")
	hitAs(e, "uid-a", "2.2.2.2")
	if w := hitAs(e, "uid-a", "3.3.3.3"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

// 經同一個 proxy（同 IP）進來的不同使用者各自有額度。
func TestRateLimit_PerUserIsolation(t *testing.T) {
	e := userRouter(0.001, 1)
	hitAs(e, "uid-b", "9.9.9.9")
	if w := hitAs(e, "uid-c", "9.9.9.9"); w.Code != http.StatusOK {
		t.Fatalf("other user status = %d, want 200", w.Code)
	}
}
