package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// rlEntry 帶最後使用時間，供閒置清理。
type rlEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit 每 IP token bucket（單 instance in-memory，ADR-010 前提下足夠）。
// 超限回 429 統一 envelope；閒置 IP 定期清理避免 map 無限成長。
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*rlEntry)

	// 清理 goroutine：每 10 分鐘移除 10 分鐘未活動的 IP
	go func() {
		for range time.Tick(10 * time.Minute) {
			mu.Lock()
			for ip, e := range entries {
				if time.Since(e.lastSeen) > 10*time.Minute {
					delete(entries, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		e, ok := entries[ip]
		if !ok {
			e = &rlEntry{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			entries[ip] = e
		}
		e.lastSeen = time.Now()
		allowed := e.limiter.Allow()
		mu.Unlock()

		if !allowed {
			response.Fail(c, apperr.New(errcode.TooManyRequests))
			return
		}
		c.Next()
	}
}
