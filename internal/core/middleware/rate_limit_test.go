package middleware

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIPRateLimiterConcurrentVisitorAccess(t *testing.T) {
	limiter := NewIPRateLimiter(5, 10)

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				ip := fmt.Sprintf("192.168.0.%d", (worker+j)%8)
				if limiter.getVisitor(ip) == nil {
					t.Errorf("expected limiter for ip %s", ip)
				}
			}
		}(i)
	}

	wg.Wait()

	if len(limiter.visitors) == 0 {
		t.Fatal("expected visitors to be tracked after concurrent access")
	}
}

func TestIPRateLimiterCleanupLoopStopsOnContextCancel(t *testing.T) {
	limiter := NewIPRateLimiter(5, 10)
	limiter.cleanupInterval = 10 * time.Millisecond
	limiter.visitorTTL = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	limiter.Start(ctx)
	cancel()

	limiter.mu.Lock()
	limiter.visitors["10.0.0.1"] = &visitor{
		lastSeen: time.Now().Add(-time.Minute),
	}
	limiter.mu.Unlock()

	time.Sleep(30 * time.Millisecond)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	if _, exists := limiter.visitors["10.0.0.1"]; !exists {
		t.Fatal("expected visitor to remain after context cancellation stopped cleanup loop")
	}
}
