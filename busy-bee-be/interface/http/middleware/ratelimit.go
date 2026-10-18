package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// rlEntry 帶最後使用時間，供閒置清理。
type rlEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter 每個 key（使用者 UID，無身分時退回 IP）一個 token bucket（單 instance in-memory，ADR-010 前提下足夠）。
// 閒置 IP 由請求觸發的惰性清理移除（無背景 goroutine，符合 goroutine 退出條件規範）。
type ipRateLimiter struct {
	mu        sync.Mutex
	entries   map[string]*rlEntry
	rps       rate.Limit
	burst     int
	idleTTL   time.Duration
	lastPrune time.Time
	now       func() time.Time // 測試注入
}

func newIPRateLimiter(rps float64, burst int, idleTTL time.Duration) *ipRateLimiter {
	return &ipRateLimiter{
		entries: make(map[string]*rlEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
		idleTTL: idleTTL,
		now:     time.Now,
	}
}

func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastPrune) >= l.idleTTL {
		for k, e := range l.entries {
			if now.Sub(e.lastSeen) > l.idleTTL {
				delete(l.entries, k)
			}
		}
		l.lastPrune = now
	}

	e, ok := l.entries[ip]
	if !ok {
		e = &rlEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.entries[ip] = e
	}
	e.lastSeen = now
	return e.limiter.Allow()
}

// RateLimit 限流 middleware；超限回 429 統一 envelope。
// 須掛在 Auth 之後：以 Firebase UID 為 key。ClientIP 取自客戶端可自填的 X-Forwarded-For，
// 以 IP 為 key 時換 header 就能換額度；只有 context 沒有身分時才退回 IP。
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	l := newIPRateLimiter(rps, burst, 10*time.Minute)
	return func(c *gin.Context) {
		key := "ip:" + c.ClientIP()
		if id, ok := user.IdentityFrom(c.Request.Context()); ok && id.UID != "" {
			key = "uid:" + id.UID
		}
		if !l.allow(key) {
			response.Fail(c, apperr.New(errcode.TooManyRequests))
			return
		}
		c.Next()
	}
}
