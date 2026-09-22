# Usage

## Constructor

```go
func New(engine *guardcore.Engine, opts ...Option) (fiberlib.Handler, error)
```

`New` returns the Fiber middleware handler. It rejects a nil engine with an
error; invalid option values are silently ignored so a bad flag can never
weaken security.

## Options

| Option | Default | Purpose |
|---|---|---|
| `WithMaxBodyBytes(int64)` | `DefaultMaxBodyBytes` (262144) | Body prefix handed to the engine for inspection |
| `WithLogger(*log.Logger)` | none | Receive fail-closed diagnostics (prefix `guardcore fiber: ...`) |

```go
guard, err := guardfiber.New(engine,
    guardfiber.WithMaxBodyBytes(64*1024),
    guardfiber.WithLogger(log.New(os.Stderr, "", log.LstdFlags)),
)
```

## Route IDs

Per-route configuration lives in the engine's `RouteRegistry`. Attach a route
ID to the Fiber user context before the guard runs by registering the ID
middleware first with `app.Use`:

```go
engine.Routes.Register("admin", func(rc *guardcore.RouteConfig) {
    rc.RequiredHeaders = guardcore.RequiredHeaders{
        {Name: "X-Admin-Token", Value: "secret"},
    }
})

routeIDs := map[string]string{"/admin/banned": "admin", "/admin/ban": "admin"}
app.Use(func(c fiberlib.Ctx) error {
    if routeID, ok := routeIDs[c.Path()]; ok {
        c.SetContext(guardfiber.WithRouteID(c.Context(), routeID))
    }
    return c.Next()
})
app.Use(guard)
```

Fiber v3 caveat: inside a use-registered middleware, `c.Route()` (and with it
`c.FullPath()`) still points at the middleware's own route entry, because the
router reassigns `c.route` for every matched tree entry as the chain executes.
Match on the concrete request path (`c.Path()`) instead; parameterized routes
need one map entry per concrete path.

## Verdicts

When the engine returns a block verdict, the middleware writes the verdict
status code, headers, and body, and never calls `c.Next()`:

| Situation | Status | Body |
|---|---|---|
| Banned IP | 403 | `IP address banned` |
| Suspicious content | 400 | `Suspicious activity detected` |
| Rate limit exceeded | 429 | `Too many requests` |

Bodies can be overridden globally through `SecurityConfig.CustomErrorResponses`.

## Fail-closed behavior

If the engine check panics, the middleware recovers, logs through the
`WithLogger` sink, and responds `500 Security check failed` rather than
letting the request through.
