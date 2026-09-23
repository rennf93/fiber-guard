// Command server assembles the advanced example: tuned engine config, the
// engine's route registry, fiber route groups, the fiber-guard middleware,
// and graceful shutdown.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	fiberlib "github.com/gofiber/fiber/v3"
	recovermw "github.com/gofiber/fiber/v3/middleware/recover"
	guardfiber "github.com/rennf93/fiber-guard"
	guardcore "github.com/rennf93/guard-core-go/v4/guardcore"

	"github.com/rennf93/fiber-guard/examples/advanced_app/internal/config"
	"github.com/rennf93/fiber-guard/examples/advanced_app/internal/routes"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	engine, err := guardcore.NewEngine(cfg)
	if err != nil {
		log.Fatalf("engine: %v", err)
	}
	if err := engine.Initialize(); err != nil {
		log.Fatalf("initialize: %v", err)
	}
	defer func() {
		if err := engine.Close(); err != nil {
			log.Printf("engine close: %v", err)
		}
	}()

	// Route registry: route IDs are attached to requests by the route-ID
	// mapper below and resolved by the engine's route_config check.
	//
	// Note on ported surface: this Go port enforces the following
	// route-scoped knobs: RequireHTTPS, MaxRequestSize,
	// AllowedContentTypes, RequiredHeaders, and authentication. Per-route
	// rate limits are NOT read by the pipeline yet; use
	// SecurityConfig.EndpointRateLimits (as done for /rate/burst) instead.
	adminToken := config.EnvOr("ADMIN_TOKEN", "admin-token-change-me")
	engine.Routes.Register("admin", func(rc *guardcore.RouteConfig) {
		rc.RequiredHeaders = guardcore.RequiredHeaders{
			{Name: "X-Admin-Token", Value: adminToken},
		}
	})

	guard, err := guardfiber.New(engine,
		guardfiber.WithMaxBodyBytes(1<<20),
		guardfiber.WithLogger(log.New(os.Stderr, "guardfiber ", log.LstdFlags)),
	)
	if err != nil {
		log.Fatalf("middleware: %v", err)
	}

	// Route IDs reach the engine through the Fiber user context
	// (guardfiber.WithRouteID, attached with c.SetContext), which the
	// adapter copies into RequestState.GuardRouteID. The mapper must
	// therefore be registered before the guard: Fiber runs middleware in
	// registration order, so a group-level middleware would run after the
	// guard and the engine would never see the route ID.
	//
	// Fiber v3 caveat: inside a use-registered middleware, c.Route() (and
	// with it c.FullPath()) still points at the middleware's own route
	// entry ("/"), because the router reassigns c.route for every matched
	// tree entry as the chain executes. The eventual handler's route is
	// therefore invisible to middleware, unlike gin/echo. Match on the
	// concrete request path (c.Path()) instead; parameterized routes need
	// one map entry per concrete path, or the pattern set expanded here.
	// Keys are exact request paths.
	routeIDs := map[string]string{
		"/admin/banned": "admin",
		"/admin/ban":    "admin",
		"/admin/unban":  "admin",
	}

	app := fiberlib.New(fiberlib.Config{
		// Server timeouts are the Fiber analog of the gin example's
		// http.Server timeouts; fasthttp applies them to every request.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	})
	// Recovery stays outermost so handler panics are contained; the mapper
	// and the guard follow so blocked requests never reach another handler.
	app.Use(recovermw.New())
	app.Use(attachRouteIDs(routeIDs))
	app.Use(guard)
	routes.Register(app, &routes.App{Engine: engine})

	// Graceful shutdown: Fiber's ListenConfig.GracefulContext makes Listen
	// stop accepting and drain in-flight requests when the context is
	// cancelled (SIGINT/SIGTERM here); ShutdownTimeout bounds the drain
	// (Fiber defaults to 10s, made explicit). engine.Close runs via defer
	// once Listen returns.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Println("advanced example listening on :8080")
	if err := app.Listen(":8080", fiberlib.ListenConfig{
		GracefulContext:       ctx,
		ShutdownTimeout:       15 * time.Second,
		DisableStartupMessage: true,
	}); err != nil {
		log.Fatalf("server: %v", err)
	}
	log.Println("shutting down")
}

// attachRouteIDs tags requests with the engine route ID of the matched
// request path before the guard runs.
func attachRouteIDs(routeIDs map[string]string) fiberlib.Handler {
	return func(c fiberlib.Ctx) error {
		if routeID, ok := routeIDs[c.Path()]; ok {
			c.SetContext(guardfiber.WithRouteID(c.Context(), routeID))
		}
		return c.Next()
	}
}
