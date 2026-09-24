# fiber-guard

Fiber middleware adapter for [guard-core-go](https://github.com/rennf93/guard-core-go). Translates `fiber.Ctx` into the guardcore request surface, runs the engine, and translates verdicts to exact Fiber responses (status, headers, body, then stop). Works with any `fiber.App` chain via `app.Use`. Unlike the net/http and Gin siblings, this adapter is fasthttp-native: Fiber runs on [fasthttp](https://github.com/valyala/fasthttp), not `net/http`, so the adapter shims `fiber.Ctx` directly.

Docs: https://rennf93.github.io/fiber-guard/

## Install

The adapter has no release tag yet; pin a commit (or track `main`) until the first tag is published:

```
go get github.com/rennf93/fiber-guard@v1.0.0 github.com/rennf93/guard-core-go/v4@v4.0.4
```

The package name is `fiber`, which collides with `github.com/gofiber/fiber/v3` (also package `fiber`), so import the adapter with an explicit alias such as `guardfiber`.

## Usage

```go
package main

import (
	"log"

	fiberlib "github.com/gofiber/fiber/v3"
	guardcore "github.com/rennf93/guard-core-go/v4/guardcore"
	guardfiber "github.com/rennf93/fiber-guard"
)

func main() {
	cfg := guardcore.DefaultSecurityConfig()
	engine, err := guardcore.NewEngine(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := engine.Initialize(); err != nil {
		log.Fatal(err)
	}

	guard, err := guardfiber.New(engine)
	if err != nil {
		log.Fatal(err)
	}

	app := fiberlib.New()
	app.Use(guard)
	app.Get("/", func(c fiberlib.Ctx) error {
		return c.SendString("ok")
	})

	log.Fatal(app.Listen(":8080"))
}
```

Options: `guardfiber.WithMaxBodyBytes(n)` bounds the body bytes the engine scans (default 262144), `guardfiber.WithLogger(l)` swaps the fail-closed logger. Route-level configuration uses `engine.Routes.Register` plus `guardfiber.WithRouteID(ctx, id)` on the Fiber user context (set `c.SetContext(...)` in a middleware registered before the guard).

Engine malfunctions fail closed with a 500. Detection covers at most the first `MaxBodyBytes` of the body; payloads beyond the bound are not scanned, and the full body still reaches your handler untouched.

Two fasthttp realities to know: the request body is fully buffered in memory before the middleware runs (Fiber's `BodyLimit` config, default 4 MiB, is the network-level bound), and `Body()` returns the Content-Encoding-decoded view, so that is what the engine scans. Client identity is the fasthttp TCP peer IP, not `c.IP()` proxy resolution.

## Development

The middleware consumes the core as a normal module dependency (`github.com/rennf93/guard-core-go/v4 v4.0.4`); no `replace` directive is used or needed. For cross-repo work on the core itself, add a temporary local `replace` line in your own checkout and drop it before committing.

Integration tests run against real Redis:

```
REDIS_HOST=127.0.0.1 go test -tags integration ./...
```

## License

MIT
