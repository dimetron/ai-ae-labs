# Idiomatic Go: Senior Developer Reference

This guide details senior-level idiomatic patterns for Go (Golang), emphasizing clarity, performance, memory efficiency, and robust concurrency.

---

## 1. Modern Error Handling

Go treats errors as regular values. Senior engineers manage error chains with clarity and avoid losing operational context.

### Wrapping Errors with `%w`

Always annotate the failure point while preserving the root cause:

```go
func (s *Service) FetchUser(ctx context.Context, id string) (*User, error) {
    user, err := s.repo.FindByID(ctx, id)
    if err != nil {
        // %w wraps the error so errors.Is and errors.As continue to work
        return nil, fmt.Errorf("fetching user %q: %w", id, err)
    }
    return user, nil
}
```

### Checking Sentinel Errors with `errors.Is`

Never use string matching (`err.Error() == "not found"`) or direct equality (`err == ErrNotFound`):

```go
var ErrUserNotFound = errors.New("user not found")

// Caller inspection
if errors.Is(err, ErrUserNotFound) {
    // Handle specific missing entity condition
}
```

### Extracting Typed Errors with `errors.As`

When an error carries domain-specific metadata (status code, validation field, retry interval):

```go
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// Caller inspection
var valErr *ValidationError
if errors.As(err, &valErr) {
    log.Printf("validation failed on field: %s, reason: %s", valErr.Field, valErr.Message)
}
```

### Combining Multiple Errors with `errors.Join` (Go 1.20+)

When performing batch teardown or multi-validation:

```go
func (c *Closer) CloseAll() error {
    var errs []error
    for _, res := range c.resources {
        if err := res.Close(); err != nil {
            errs = append(errs, err)
        }
    }
    return errors.Join(errs...)
}
```

### Error Handling Anti-Patterns
- **Logging and returning**: Log it OR return it. Doing both fills logs with duplicate traces for a single failure.
- **Blind ignoring**: Never do `_ = file.Close()`. Check or explicitly comment why an error is ignored (e.g., in a defer block: `defer func() { _ = res.Body.Close() }()`).

---

## 2. Concurrency & Goroutine Lifecycle Management

Concurrency is one of Go's primary strengths, but unmanaged goroutines cause catastrophic resource leaks.

### The Lifetime Rule
> **Never start a goroutine without knowing exactly how, why, and when it will terminate.**

### Context Propagation Rules
1. **Pass `ctx` as the first parameter**: `func Query(ctx context.Context, ...)`.
2. **Never store `context.Context` inside a struct**: Storing `ctx` obscures its lifecycle and breaks cancellation propagation.
3. **Always honor cancellation**: Periodically check `ctx.Err()` or include `ctx.Done()` in `select` blocks.

```go
func StreamEvents(ctx context.Context, ch <-chan Event) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case ev, ok := <-ch:
            if !ok {
                return nil // Channel closed cleanly
            }
            process(ev)
        }
    }
}
```

### Structured Concurrency with `errgroup`

Instead of raw `sync.WaitGroup` with error tracking channels, use `golang.org/x/sync/errgroup`:

```go
import "golang.org/x/sync/errgroup"

func ProcessBatch(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)
    // Optional: limit concurrent goroutines
    g.SetLimit(10)

    for _, item := range items {
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    // Wait blocks until all goroutines finish or the first error occurs
    return g.Wait()
}
```

### Thread-Safe Initialization with `sync.Once`

```go
type ClientPool struct {
    client *Client
    once   sync.Once
}

func (p *ClientPool) Get() *Client {
    p.once.Do(func() {
        p.client = newClient()
    })
    return p.client
}
```

---

## 3. Memory, Slices & Performance

### Slice Pre-allocation

Avoid dynamic slice reallocations by setting capacity upfront:

```go
// ❌ Poor: triggers multiple internal reallocations and array copying
var results []Result
for _, item := range items {
    results = append(results, transform(item))
}

// ✅ Idiomatic: allocate exact capacity once
results := make([]Result, 0, len(items))
for _, item := range items {
    results = append(results, transform(item))
}
```

### Preventing Sub-Slice Memory Leaks

Reslicing a small portion of a huge slice retains the entire underlying backing array in memory:

```go
// ❌ Leaks memory: keeps the entire 100MB file buffer alive
func GetHeader(hugeBuffer []byte) []byte {
    return hugeBuffer[:16]
}

// ✅ Idiomatic: allocate a new slice and copy only what is needed
func GetHeader(hugeBuffer []byte) []byte {
    header := make([]byte, 16)
    copy(header, hugeBuffer[:16])
    return header
}
```

### Map Allocation Hints

If the size of a map is known or estimable, supply the capacity hint:

```go
lookup := make(map[string]User, len(users))
```

### Struct Field Alignment & Memory Padding

The Go compiler aligns struct fields on word boundaries (8 bytes on 64-bit platforms). Reordering fields saves significant memory across millions of allocations:

```go
// ❌ Poor layout: 24 bytes (with padding)
type BadStruct struct {
    a bool   // 1 byte + 7 bytes padding
    b int64  // 8 bytes
    c bool   // 1 byte + 7 bytes padding
}

// ✅ Optimized layout: 16 bytes (grouped by size)
type GoodStruct struct {
    b int64  // 8 bytes
    a bool   // 1 byte
    c bool   // 1 byte + 6 bytes padding
}
```

---

## 4. Generics (Go 1.18+) Best Practices

### When to Use Generics
- **Generic Data Structures**: Binary search trees, linked lists, caches, concurrent rings.
- **Slice & Map Utilities**: Deduplication, mapping, filtering, extracting keys.

```go
// Clean slice transformation utility
func Map[T any, R any](items []T, fn func(T) R) []R {
    result := make([]R, len(items))
    for i, v := range items {
        result[i] = fn(v)
    }
    return result
}
```

### When NOT to Use Generics
- Do not use generics when interfaces are sufficient (`io.Reader`, `fmt.Stringer`).
- Do not use generics in business domain models to simulate OOP class templates.
- If you find yourself writing `[T any]` on a business service struct (`UserService[T]`), step back and return to concrete types and interfaces.
