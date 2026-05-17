package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	"sports-dashboard/internal/core/exceptions"
	"sports-dashboard/internal/shared/schemas"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	rps             float64
	burst           int
	visitors        map[string]*visitor
	mu              sync.Mutex
	startOnce       sync.Once
	cleanupInterval time.Duration
	visitorTTL      time.Duration
}

func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		rps:             rps,
		burst:           burst,
		visitors:        make(map[string]*visitor),
		cleanupInterval: time.Minute,
		visitorTTL:      3 * time.Minute,
	}
}

func (l *IPRateLimiter) Start(ctx context.Context) {
	l.startOnce.Do(func() {
		go l.cleanupLoop(ctx)
	})
}

func (l *IPRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := l.getVisitor(ip)
		if !limiter.Allow() {
			c.Header("Retry-After", "60")
			schemas.Error(c, http.StatusTooManyRequests, "Too many requests", exceptions.RATE_LIMITED, nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func (l *IPRateLimiter) getVisitor(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, exists := l.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(l.rps), l.burst)
		l.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (l *IPRateLimiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.cleanupExpiredVisitors()
		}
	}
}

func (l *IPRateLimiter) cleanupExpiredVisitors() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, v := range l.visitors {
		if now.Sub(v.lastSeen) > l.visitorTTL {
			delete(l.visitors, ip)
		}
	}
}
