// Package routes holds the HTTP handlers of the advanced example, split by
// concern the way the Python advanced example splits routers. The guard
// middleware and the route-ID mapper are attached by cmd/server before these
// handlers run.
package routes

import (
	"encoding/json"
	"log"

	fiberlib "github.com/gofiber/fiber/v3"

	guardcore "github.com/rennf93/guard-core-go/v4/guardcore"
)

// App carries the engine into handlers that need operational access (the
// admin routes drive the ban manager directly).
type App struct {
	Engine *guardcore.Engine
}

// Register builds the full route table on the given app. The /admin/* group
// is registered on the engine's RouteRegistry with a RequiredHeaders guard
// (see cmd/server/main.go): the engine itself rejects calls missing the
// admin token with a 400 before these handlers run.
func Register(app *fiberlib.App, a *App) {
	app.Get("/", a.info)
	app.Get("/health", a.health)
	app.Get("/ready", a.ready)
	app.Post("/echo", a.echo)
	app.Get("/rate/burst", a.burst)
	app.Get("/test/xss", a.attackEcho)
	app.Get("/test/sqli", a.attackEcho)

	admin := app.Group("/admin")
	admin.Get("/banned", a.bannedCount)
	admin.Post("/ban", a.ban)
	admin.Post("/unban", a.unban)
}

// health and ready are excluded from the pipeline (config.ExcludePaths), so
// probes and orchestrator health checks never trip the guard.
func (a *App) health(c fiberlib.Ctx) error {
	return c.JSON(fiberlib.Map{"status": "ok"})
}

func (a *App) ready(c fiberlib.Ctx) error {
	// Extend this with real dependency probes (Redis PING, cloud range
	// warmup) for your deployment.
	return c.JSON(fiberlib.Map{"status": "ready"})
}

func (a *App) info(c fiberlib.Ctx) error {
	return c.JSON(fiberlib.Map{
		"app":     "fiber-guard advanced example",
		"routes":  []string{"/health", "/ready", "/echo", "/rate/burst", "/admin/*", "/test/*"},
		"engine":  "guardcore",
		"version": "1.0.0",
	})
}

// echo proves the adapter's body contract: fasthttp buffers the full body
// before the middleware runs, the engine scans only the bounded prefix, and
// the handler reads the full, untouched body straight from Fiber.
func (a *App) echo(c fiberlib.Ctx) error {
	body := c.Body()
	return c.JSON(fiberlib.Map{
		"echo":   true,
		"method": c.Method(),
		"path":   c.Path(),
		"bytes":  len(body),
	})
}

// burst is limited by config.EndpointRateLimits (5 requests per 60 seconds),
// which this port enforces by exact request path.
func (a *App) burst(c fiberlib.Ctx) error {
	return c.JSON(fiberlib.Map{
		"endpoint": "/rate/burst",
		"limit":    "5 requests per 60 seconds",
	})
}

// The /admin/* routes are registered on the engine's RouteRegistry with a
// RequiredHeaders guard (see cmd/server/main.go): the engine itself rejects
// calls missing the admin token with a 400 before these handlers run.

func (a *App) bannedCount(c fiberlib.Ctx) error {
	return c.JSON(fiberlib.Map{
		"banned_ips":      a.Engine.Ban.BannedIPCount(),
		"banned_networks": a.Engine.Ban.BannedNetworkCount(),
	})
}

func (a *App) ban(c fiberlib.Ctx) error {
	var body struct {
		IP      string `json:"ip"`
		Seconds int    `json:"seconds"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || body.IP == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{
			"error": `body must be {"ip": ..., "seconds": ..., "reason": ...}`,
		})
	}
	if body.Seconds <= 0 {
		body.Seconds = 300
	}
	if body.Reason == "" {
		body.Reason = "manual ban via admin route"
	}
	created, err := a.Engine.Ban.Ban(body.IP, body.Seconds, body.Reason)
	if err != nil {
		log.Printf("admin ban %s failed: %v", body.IP, err)
		return c.Status(fiberlib.StatusInternalServerError).JSON(fiberlib.Map{"error": "ban failed"})
	}
	return c.JSON(fiberlib.Map{
		"ip": body.IP, "seconds": body.Seconds, "created": created,
	})
}

func (a *App) unban(c fiberlib.Ctx) error {
	var body struct {
		IP string `json:"ip"`
	}
	if err := json.Unmarshal(c.Body(), &body); err != nil || body.IP == "" {
		return c.Status(fiberlib.StatusBadRequest).JSON(fiberlib.Map{"error": `body must be {"ip": ...}`})
	}
	if err := a.Engine.Ban.Unban(body.IP); err != nil {
		log.Printf("admin unban %s failed: %v", body.IP, err)
		return c.Status(fiberlib.StatusInternalServerError).JSON(fiberlib.Map{"error": "unban failed"})
	}
	return c.JSON(fiberlib.Map{"ip": body.IP, "status": "unbanned"})
}

// attackEcho handlers intentionally echo hostile payloads; the guard blocks
// the request before the handler runs, so reaching this code means detection
// was bypassed (defense in depth: do not log or store the payload).
// Payloads ride in query parameters because this port does not scan request
// bodies yet (see the engine port's roadmap notes).
func (a *App) attackEcho(c fiberlib.Ctx) error {
	return c.JSON(fiberlib.Map{"detected": false})
}
