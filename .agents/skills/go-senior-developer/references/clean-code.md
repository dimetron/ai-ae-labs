# Clean Code & Architecture in Go: Senior Developer Reference

This guide establishes conventions for package design, project structure, interface ergonomics, and code hygiene in Go systems.

---

## 1. Standard Project Layout

Go projects should follow the standard structure to keep application boundaries rigid:

```text
my-project/
├── cmd/
│   └── api/
│       └── main.go         # Composition root: wires dependencies, starts server
├── internal/               # Private application code (enforced by Go compiler)
│   ├── order/              # Business domain: Order entity, logic, repository interface
│   │   ├── order.go
│   │   ├── service.go
│   │   └── service_test.go
│   ├── platform/           # Reusable internal drivers and infrastructure
│   │   ├── database/
│   │   └── telemetry/
│   └── transport/
│       ├── http/           # HTTP handlers, routing, middleware
│       └── grpc/           # gRPC server implementations
├── pkg/                    # Optional: Public libraries safe for external module import
├── api/                    # OpenAPI specs, protobuf/gRPC definitions
├── go.mod
└── go.sum
```

### The Rule of `internal/`
Place the vast majority of application code under `internal/`. The Go compiler strictly forbids any other repository or module from importing packages located inside `internal/`. This shields your internals from becoming external API commitments.

---

## 2. Interface Segregation: Accept Interfaces, Return Structs

```
   Consumer Package                          Producer Package
┌───────────────────────────┐             ┌─────────────────────────┐
│ type Reader interface {   │             │ type FileStore struct { │
│     Read(p []byte) (n, err│◄────────────┼─  // Concrete           │
│ }                         │ Implemented │ }                       │
│                           │ By          │                         │
│ func Process(r Reader)    │             │ func New() *FileStore   │
└───────────────────────────┘             └─────────────────────────┘
```

### Guidelines:
1. **Define interfaces where they are consumed**: The package calling the interface defines the minimal method set it requires.
2. **Return concrete structs from constructors**: Constructors (`New...`) should return a pointer to a concrete struct (`*Service`), not an interface (`Service`). This allows adding new methods to the struct in the future without breaking callers.
3. **Keep interfaces small (1–2 methods)**: Small interfaces (`io.Reader`, `io.Closer`, `fmt.Stringer`) compose easily and are trivial to mock.

```go
// ❌ Bad: 15-method interface defined in provider package
package store
type BigStore interface {
    GetUser(...)
    SaveUser(...)
    DeleteUser(...)
    GetOrder(...)
    // ...
}

// ✅ Good: consumer defines exactly what it needs
package order
type UserFinder interface {
    FindUser(ctx context.Context, id string) (*User, error)
}

type Service struct {
    users UserFinder
}
```

---

## 3. Banishing Anti-Patterns

### Anti-Pattern: "Garbage-Can" Packages (`util`, `common`, `helpers`)
Never create packages named `util`, `common`, or `helpers`. These become unstructured dumping grounds with circular dependency issues.

- Instead of `util.FormatDate()` → Create `dateutil.Format()` or place in domain package.
- Instead of `common.User` → Place in `internal/user`.
- Instead of `helpers.HTTPResponse` → Place in `internal/transport/httputil`.

### Anti-Pattern: Package Name Stutter
Avoid repeating the package name in struct or function identifiers:

```go
// ❌ Stutter
package user
type UserService struct{} // Called as user.UserService
func NewUserService() *UserService

// ✅ Idiomatic
package user
type Service struct{}     // Called as user.Service
func NewService() *Service
```

---

## 4. Making the Zero Value Useful

A well-designed Go struct should be ready to use immediately without calling an explicit initialization function whenever possible.

### Example: Zero-Value Usable Buffer / Counter

```go
type Counter struct {
    mu    sync.Mutex
    count int64
}

// Increment works safely on a zero-value Counter (var c Counter)
func (c *Counter) Increment() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
    return c.count
}
```

### Lazy Initialization on First Use

If a field requires expensive setup, initialize it on first access rather than forcing a mandatory constructor:

```go
type Logger struct {
    out io.Writer
}

func (l *Logger) Write(p []byte) (n int, err error) {
    w := l.out
    if w == nil {
        w = os.Stdout // Sensible zero-value default
    }
    return w.Write(p)
}
```
