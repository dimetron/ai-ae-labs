---
name: go-senior-developer
description: Use when designing, writing, reviewing, testing, or refactoring Go (Golang) code, implementing idiomatic Go patterns, setting up TDD with table-driven tests, securing Go services, or designing clean package architectures.
---

# Go Senior Developer Skill

A comprehensive operational manual and reference guide for writing production-grade, idiomatic, secure, and clean Go (Golang) systems.

---

## The Senior Go Mindset

1. **Clear is better than clever**: Avoid premature abstractions, deeply nested inheritance-like structs, or unnecessary generics.
2. **Accept interfaces, return structs**: Consumers define the abstractions they need. Producers return concrete, initialized values.
3. **Make the zero value useful**: Design structs such that default initialization (`var s State`) is immediately functional without panics.
4. **Never start a goroutine without knowing how it stops**: Every concurrent operation must have an explicit owner, a shutdown trigger, and context cancellation handling.
5. **Handle errors once, wrap with context**: Use `fmt.Errorf("...: %w", err)` to preserve the error chain. Never log an error and then return it.
6. **No "garbage-can" packages**: Strictly avoid `pkg/util`, `pkg/common`, or `pkg/helpers`. Group packages by cohesive business or technical domain.

---

## High-Impact Decision Matrices

### 1. Concurrency: Channels vs. `sync.Mutex`

| Scenario | Preferred Primitive | Rationale |
| :--- | :--- | :--- |
| **Passing ownership of data** | Channel | Communicates across boundaries; prevents shared mutable state. |
| **Distributing tasks / Pipeline** | Channel | Idiomatic worker pools, fan-out/fan-in, streaming pipelines. |
| **Coordinating cancellation / signals** | `context.Context` / Channel | Cancellation broadcast (`ctx.Done()`) or signal notifications. |
| **Internal struct state / Cache** | `sync.Mutex` / `sync.RWMutex` | Faster, simpler for in-memory dictionaries and state machines. |
| **Single one-time initialization** | `sync.Once` | Guaranteed thread-safe lazy init without double-checked locking bugs. |

### 2. Method Receivers: Value (`(t T)`) vs. Pointer (`(t *T)`)

- **Use Pointer Receiver (`*T`) when:**
  - The method needs to mutate the receiver's state.
  - The struct contains synchronization primitives (`sync.Mutex`, `sync.WaitGroup`) which **must never be copied**.
  - The struct is large and copying it on each method call would impose high memory overhead.
  - Consistency: If any method of `T` requires a pointer receiver, make *all* methods of `T` use pointer receivers.
- **Use Value Receiver (`T`) when:**
  - The type is small and immutable (e.g. `time.Time`, coordinate points, basic enums).
  - The type is a map, channel, or function type (already reference descriptors).

### 3. Error Representation

| Requirement | Approach | Example |
| :--- | :--- | :--- |
| **Simple contextual failure** | Wrap existing error with `%w` | `fmt.Errorf("fetching user %s: %w", id, err)` |
| **Known static condition** | Package-level sentinel error (`var Err... = errors.New(...)`) | `errors.Is(err, sql.ErrNoRows)` |
| **Error carries structured metadata** | Custom error struct implementing `Error() string` | `var valErr *ValidationError; errors.As(err, &valErr)` |
| **Multiple independent failures** | Combine via `errors.Join` (Go 1.20+) | `return errors.Join(err1, err2)` |

---

## Senior Go Code Review Rubric

When reviewing or refactoring Go code, evaluate against this checklist:

- [ ] **Error Handling**:
  - Errors are wrapped with `%w` instead of `%v` or `%s` unless hiding internal details at the boundary.
  - Errors are handled exactly once (not logged *and* returned).
  - Callers check errors immediately (`if err != nil`). No swallowed errors (`_ = fn()`).
- [ ] **Goroutine & Context Hygiene**:
  - `context.Context` is the first parameter of I/O operations (`ctx context.Context`).
  - Context is never stored inside a struct.
  - Goroutines either listen to `ctx.Done()` or use a deterministic `sync.WaitGroup` / `errgroup.Group`.
  - Background goroutines do not outlive their originating request or service lifecycle.
- [ ] **Interface & Package Cleanliness**:
  - Interfaces are small (1–2 methods, adhering to Interface Segregation).
  - Interfaces are declared where consumed (client package), not where implemented (provider package).
  - No package name stutter (e.g. `user.NewService`, not `user.NewUserService`).
  - No `pkg/util` or `pkg/common`.
- [ ] **Safety & Security**:
  - All database queries use parameter placeholders (`$1`, `?`); zero string concatenation in SQL.
  - External URLs and file paths are strictly sanitized (`filepath.Clean`) against path traversal and SSRF.
  - Cryptographic randomness uses `crypto/rand`, never `math/rand`.
  - Code runs clean under `go test -race ./...`.

---

## Detailed Reference Modules

Consult these dedicated guides for in-depth examples, patterns, and runbooks:

- [Idiomatic Go Guide](references/idiomatic-go.md): Error wrapping, memory/slice optimizations, goroutine lifecycle, generics.
- [TDD & Testing Standards](references/tdd-testing.md): Red-Green-Refactor, table-driven tests, consumer mocking, fuzzing, benchmarks.
- [Go Design Patterns](references/design-patterns.md): Functional options, middleware decorators, hexagonal architecture, graceful shutdown.
- [Go Security Manual](references/security.md): Parameterized SQL, SSRF & path traversal defense, crypto hygiene, `govulncheck`.
- [Clean Code & Architecture](references/clean-code.md): Standard project layout, zero-value usability, interface ergonomics.

## Production-Grade Examples

Inspect runnable reference implementations:
- [Functional Options Pattern](examples/functional_options_test.go)
- [Table-Driven TDD with Consumer Mocks](examples/table_driven_tdd_test.go)
- [Graceful Shutdown Engine](examples/graceful_shutdown.go)
