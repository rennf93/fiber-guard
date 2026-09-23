//go:build integration

package fiber

import (
	"os"
	"testing"

	fiberlib "github.com/gofiber/fiber/v3"

	"github.com/rennf93/guard-core-go/v4/guardcore"
)

func newIntegrationMiddleware(t *testing.T) (fiberlib.Handler, *guardcore.Engine) {
	t.Helper()
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		t.Skip("REDIS_HOST not set")
	}
	cfg := guardcore.DefaultSecurityConfig()
	cfg.EnableRedis = true
	cfg.RedisURL = "redis://" + host + ":6379"
	cfg.RedisPrefix = "guard_core_fiber_test:"
	cfg.RedisFailOpen = false
	engine, err := guardcore.NewEngine(cfg)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	if err := engine.Initialize(); err != nil {
		t.Fatalf("engine initialize: %v", err)
	}
	guard, err := New(engine)
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	t.Cleanup(func() {
		_, _ = engine.Redis.DeletePattern("banned_ips:*")
		_, _ = engine.Redis.DeletePattern("rate_limit:rate:*")
		_ = engine.Close()
	})
	return guard, engine
}

func TestIntegrationMiddlewareBlocksBannedIP(t *testing.T) {
	guard, engine := newIntegrationMiddleware(t)
	bannedIP := "203.0.113.60"
	applied, err := engine.Ban.Ban(bannedIP, 120, "integration-test")
	if err != nil || !applied {
		t.Fatalf("ban not applied: %v %v", applied, err)
	}
	req := newReq("GET", "/api", "", nil)
	resp, p := serve(t, guard, bannedIP+":4711", req)
	assertBlockedResponse(t, resp, 403, guardcore.IPBanBlockedMessage)
	if p.called {
		t.Fatal("blocked request must not reach the handler")
	}
}

func TestIntegrationMiddlewareStartupWiringAcrossEngines(t *testing.T) {
	_, first := newIntegrationMiddleware(t)
	bannedIP := "203.0.113.61"
	applied, err := first.Ban.Ban(bannedIP, 120, "integration-test")
	if err != nil || !applied {
		t.Fatalf("ban not applied: %v %v", applied, err)
	}
	secondGuard, _ := newIntegrationMiddleware(t)
	blocked := newReq("GET", "/api", "", nil)
	resp, p := serve(t, secondGuard, bannedIP+":4711", blocked)
	assertBlockedResponse(t, resp, 403, guardcore.IPBanBlockedMessage)
	if p.called {
		t.Fatal("second engine must see the redis-backed ban")
	}
	allowed := newReq("GET", "/api", "", nil)
	resp, p = serve(t, secondGuard, "203.0.113.62:4711", allowed)
	if resp.StatusCode() != 200 || !p.called {
		t.Fatalf("unbanned IP must pass through the second engine, got %d called=%v", resp.StatusCode(), p.called)
	}
}

func TestIntegrationMiddlewareRateLimitSharedAcrossEngines(t *testing.T) {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		t.Skip("REDIS_HOST not set")
	}
	cfg := guardcore.DefaultSecurityConfig()
	cfg.EnableRedis = true
	cfg.RedisURL = "redis://" + host + ":6379"
	cfg.RedisPrefix = "guard_core_fiber_test:"
	cfg.RedisFailOpen = false
	cfg.RateLimit = 2
	cfg.RateLimitWindow = 60
	engine, err := guardcore.NewEngine(cfg)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	if err := engine.Initialize(); err != nil {
		t.Fatalf("engine initialize: %v", err)
	}
	guard, err := New(engine)
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}
	t.Cleanup(func() {
		_, _ = engine.Redis.DeletePattern("banned_ips:*")
		_, _ = engine.Redis.DeletePattern("rate_limit:rate:*")
		_ = engine.Close()
	})
	ratelimitedIP := "203.0.113.63"
	for i := 0; i < 2; i++ {
		req := newReq("GET", "/api", "", nil)
		resp, p := serve(t, guard, ratelimitedIP+":4711", req)
		if resp.StatusCode() != 200 || !p.called {
			t.Fatalf("request %d within the limit must pass, got %d called=%v", i+1, resp.StatusCode(), p.called)
		}
	}
	third := newReq("GET", "/api", "", nil)
	resp, p := serve(t, guard, ratelimitedIP+":4711", third)
	assertBlockedResponse(t, resp, 429, "Too many requests")
	if p.called {
		t.Fatal("third request must be rate limited")
	}
}
