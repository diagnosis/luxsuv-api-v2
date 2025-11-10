package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/diagnosis/luxsuv-api-v2/internal/apperror"
	"github.com/diagnosis/luxsuv-api-v2/internal/helper"
	"github.com/diagnosis/luxsuv-api-v2/internal/logger"
)

type visitor struct {
	tokens   int
	lastSeen time.Time
	mu       sync.Mutex
}

type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     int
	window   time.Duration
	cleanup  time.Duration
}

func NewRateLimiter(requestsPerWindow int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     requestsPerWindow,
		window:   window,
		cleanup:  window * 2,
	}

	go rl.cleanupRoutine()

	return rl
}

func (rl *RateLimiter) getVisitor(ip string) *visitor {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		v = &visitor{
			tokens:   rl.rate,
			lastSeen: time.Now(),
		}
		rl.visitors[ip] = v
	}

	return v
}

func (rl *RateLimiter) allow(ip string) bool {
	v := rl.getVisitor(ip)

	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(v.lastSeen)

	if elapsed > rl.window {
		v.tokens = rl.rate
		v.lastSeen = now
	}

	if v.tokens > 0 {
		v.tokens--
		v.lastSeen = now
		return true
	}

	return false
}

func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup_old_visitors()
	}
}

func (rl *RateLimiter) cleanup_old_visitors() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, v := range rl.visitors {
		v.mu.Lock()
		if now.Sub(v.lastSeen) > rl.cleanup {
			delete(rl.visitors, ip)
		}
		v.mu.Unlock()
	}
}

func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			ip := helper.ClientIP(r)

			if !rl.allow(ip) {
				logger.Warn(ctx, "rate limit exceeded", "ip", ip)
				helper.RespondError(w, r, apperror.TooManyRequests("Rate limit exceeded. Please try again later."))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
