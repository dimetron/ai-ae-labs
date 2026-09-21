# Go Design Patterns: Senior Developer Reference

Go favors composition and simple interfaces over classical object-oriented class hierarchies. This guide covers idiomatic design patterns in Go.

---

## 1. Functional Options Pattern

The Functional Options pattern is the gold standard in Go for constructing objects with sensible defaults and optional parameters.

### Implementation

```go
package server

import (
    "errors"
    "time"
)

type Server struct {
    host    string
    port    int
    timeout time.Duration
    maxConn int
}

// Option represents a functional configuration mutator
type Option func(*Server) error

func WithTimeout(d time.Duration) Option {
    return func(s *Server) error {
        if d <= 0 {
            return errors.New("timeout must be greater than zero")
        }
        s.timeout = d
        return nil
    }
}

func WithMaxConnections(n int) Option {
    return func(s *Server) error {
        if n <= 0 {
            return errors.New("max connections must be positive")
        }
        s.maxConn = n
        return nil
    }
}

// New constructs a Server with defaults and applied options
func New(host string, port int, opts ...Option) (*Server, error) {
    s := &Server{
        host:    host,
        port:    port,
        timeout: 30 * time.Second, // Default
        maxConn: 100,              // Default
    }

    for _, opt := range opts {
        if err := opt(s); err != nil {
            return nil, err
        }
    }

    return s, nil
}
```

---

## 2. Middleware & Decorator Pattern

In Go, middleware wraps functions and interfaces to add cross-cutting concerns (authentication, telemetry, rate limiting, logging) without modifying business logic.

### HTTP Middleware Pattern

```go
type Middleware func(http.Handler) http.Handler

func LoggingMiddleware(logger *log.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            next.ServeHTTP(w, r)
            logger.Printf("%s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
        })
    }
}

// Chaining middlewares
func Chain(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}
```

### Domain Interface Decorator

Decorators can wrap any domain interface:

```go
type OrderService interface {
    PlaceOrder(ctx context.Context, order Order) error
}

type metricsDecorator struct {
    base OrderService
}

func (m *metricsDecorator) PlaceOrder(ctx context.Context, order Order) error {
    start := time.Now()
    err := m.base.PlaceOrder(ctx, order)
    metrics.RecordLatency("order.place", time.Since(start), err == nil)
    return err
}
```

---

## 3. Hexagonal Architecture (Ports & Adapters)

Hexagonal architecture organizes Go services so business logic remains pure and decoupled from frameworks, transport layers, and databases.

```
                  ┌─────────────────────────────────────┐
                  │           Adapters (HTTP)           │
                  │  (internal/adapter/http/handler.go) │
                  └──────────────────┬──────────────────┘
                                     │
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Core Domain / Service (internal/domain/order.go)                       │
│                                                                        │
│   type Order struct { ID string; Amount int64 }                       │
│                                                                        │
│   // Port (Interface defined by core)                                  │
│   type Repository interface {                                          │
│       Save(ctx context.Context, order Order) error                     │
│   }                                                                    │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │
                                     ▼
                  ┌─────────────────────────────────────┐
                  │         Adapters (Database)         │
                  │ (internal/adapter/postgres/repo.go) │
                  └─────────────────────────────────────┘
```

**Key Senior Rules:**
1. The **Domain** imports nothing from `adapter`, `http`, `sql`, or third-party ORMs.
2. The **Ports** are defined by the consumer (domain service).
3. The **Adapters** implement those ports. `main.go` acts as the composition root, wiring adapters into domain services.

---

## 4. Concurrency Patterns

### Bounded Worker Pool

Process jobs concurrently while strictly bounding the number of active goroutines:

```go
func RunWorkerPool(ctx context.Context, numWorkers int, jobs <-chan Job) <-chan Result {
    results := make(chan Result)
    var wg sync.WaitGroup

    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case job, ok := <-jobs:
                    if !ok {
                        return
                    }
                    res := process(job)
                    select {
                    case <-ctx.Done():
                        return
                    case results <- res:
                    }
                }
            }
        }()
    }

    // Closer goroutine: closes results channel once all workers exit
    go func() {
        wg.Wait()
        close(results)
    }()

    return results
}
```

### Fan-Out / Fan-In

```go
func Merge[T any](ctx context.Context, channels ...<-chan T) <-chan T {
    out := make(chan T)
    var wg sync.WaitGroup

    output := func(c <-chan T) {
        defer wg.Done()
        for v := range c {
            select {
            case <-ctx.Done():
                return
            case out <- v:
            }
        }
    }

    wg.Add(len(channels))
    for _, c := range channels {
        go output(c)
    }

    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}
```

---

## 5. Graceful Shutdown

Production services must intercept OS termination signals and cleanly drain connections.

```go
func Run(ctx context.Context) error {
    // 1. Trap SIGINT and SIGTERM
    ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
    defer cancel()

    srv := &http.Server{
        Addr:         ":8080",
        Handler:      router,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
    }

    // 2. Start server in a background goroutine
    serverErr := make(chan error, 1)
    go func() {
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            serverErr <- err
        }
        close(serverErr)
    }()

    // 3. Block until signal received or server error
    select {
    case err := <-serverErr:
        return fmt.Errorf("server crashed: %w", err)
    case <-ctx.Done():
        log.Println("shutting down gracefully...")
    }

    // 4. Drain connections with a hard deadline
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer shutdownCancel()

    if err := srv.Shutdown(shutdownCtx); err != nil {
        return fmt.Errorf("server forced to shutdown: %w", err)
    }

    log.Println("server exited cleanly")
    return nil
}
```
