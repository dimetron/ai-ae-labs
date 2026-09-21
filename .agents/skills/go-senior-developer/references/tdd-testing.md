# Test-Driven Development (TDD) & Testing Standards in Go

This guide outlines test-driven development (TDD), table-driven test conventions, consumer-driven mocking, and testing tooling for Go.

---

## 1. The Red-Green-Refactor Cycle in Go

In Go, TDD drives clean interface boundaries and eliminates unnecessary coupling.

```
       [ 1. RED ]
  Write a failing test
  defining the expected
  API and behavior
           │
           ▼
      [ 2. GREEN ]
  Implement the minimal,
  working Go code to satisfy
  the test
           │
           ▼
     [ 3. REFACTOR ]
  Optimize allocations, clean
  package layout, ensure zero
  data races (-race)
```

1. **RED**: Write a table-driven test or test double defining what the unit should do before writing the implementation. Run `go test` and confirm it fails for the expected reason (e.g., function not implemented or expected error mismatch).
2. **GREEN**: Write only enough code to pass the test. Do not design speculative features.
3. **REFACTOR**: Simplify code structure, verify interface boundaries, ensure error handling is wrapped with `%w`, and run `go test -race ./...`.

---

## 2. Idiomatic Table-Driven Tests

Table-driven testing is the standard testing pattern in Go. It makes adding new edge cases trivial and presents test intentions clearly.

### Standard Template

```go
package user_test

import (
    "context"
    "errors"
    "testing"

    "myproject/internal/user"
)

func TestService_FindUser(t *testing.T) {
    t.Parallel() // Enables parallel execution for the parent test suite

    type fields struct {
        store user.Store
    }
    type args struct {
        ctx context.Context
        id  string
    }

    tests := []struct {
        name        string
        fields      fields
        args        args
        want        *user.User
        wantErr     bool
        expectedErr error
    }{
        {
            name: "success: valid user ID returns user",
            fields: fields{
                store: &mockStore{
                    findByIDFunc: func(ctx context.Context, id string) (*user.User, error) {
                        return &user.User{ID: "usr_123", Name: "Alice"}, nil
                    },
                },
            },
            args: args{
                ctx: context.Background(),
                id:  "usr_123",
            },
            want:    &user.User{ID: "usr_123", Name: "Alice"},
            wantErr: false,
        },
        {
            name: "error: empty user ID returns validation error",
            fields: fields{
                store: &mockStore{},
            },
            args: args{
                ctx: context.Background(),
                id:  "",
            },
            want:        nil,
            wantErr:     true,
            expectedErr: user.ErrInvalidID,
        },
    }

    for _, tt := range tests {
        // Go 1.22+ scopes loop variables per iteration; in earlier versions, use tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Subtests run in parallel

            svc := user.NewService(tt.fields.store)
            got, err := svc.FindUser(tt.args.ctx, tt.args.id)

            if (err != nil) != tt.wantErr {
                t.Fatalf("FindUser() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
                t.Fatalf("FindUser() error = %v, want error %v", err, tt.expectedErr)
            }
            if !tt.wantErr && got.ID != tt.want.ID {
                t.Errorf("FindUser() got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

---

## 3. Test Lifecycle & Helpers

### `t.Helper()`
Mark test helper functions with `t.Helper()`. This ensures failure stack traces report the caller line instead of the helper's internal line:

```go
func assertUserEqual(t *testing.T, got, want *User) {
    t.Helper()
    if got.ID != want.ID || got.Email != want.Email {
        t.Errorf("User mismatch: got %+v, want %+v", got, want)
    }
}
```

### `t.Cleanup()`
Use `t.Cleanup()` instead of `defer` when creating test fixtures. `t.Cleanup()` executes reliably after the test completes, even during subtests or unexpected panics:

```go
func createTempDB(t *testing.T) *sql.DB {
    t.Helper()
    dir := t.TempDir() // Automatically removed when test finishes
    db, err := sql.Open("sqlite3", filepath.Join(dir, "test.db"))
    if err != nil {
        t.Fatalf("failed to open sqlite: %v", err)
    }

    t.Cleanup(func() {
        _ = db.Close()
    })

    return db
}
```

---

## 4. Consumer-Driven Mocking Without Framework Bloat

Heavy code generators (`mockery`, `gomock`) often introduce bloated generated code and fragile test expectations.

### Idiomatic Function-Based Test Doubles

Define the minimal interface where the service is consumed, then implement a struct with function fields:

```go
// In consumer package (e.g. internal/order)
type PaymentGateway interface {
    Charge(ctx context.Context, amount int64) (string, error)
}

// In test file (order_test.go)
type mockPaymentGateway struct {
    chargeFunc func(ctx context.Context, amount int64) (string, error)
}

func (m *mockPaymentGateway) Charge(ctx context.Context, amount int64) (string, error) {
    if m.chargeFunc == nil {
        panic("chargeFunc unexpectedly invoked without mock definition")
    }
    return m.chargeFunc(ctx, amount)
}
```

**Benefits:**
- Zero external code generation dependencies.
- Each test case customizes only the behavior it cares about.
- Strict panic safety if an unexpected method is called.

---

## 5. Senior Testing Toolset

### Race Detection
Always verify concurrent code with the race detector:
```bash
go test -race ./...
```

### Fuzz Testing (`testing.F`)
Fuzz tests generate random inputs to discover crashes, memory leaks, and panic edge cases:

```go
func FuzzParseJSON(f *testing.F) {
    // Seed the corpus
    f.Add([]byte(`{"name":"test","age":25}`))
    f.Add([]byte(`{}`))

    f.Fuzz(func(t *testing.T, data []byte) {
        // Parse should either succeed or return a clean error, never panic
        _, _ = ParseUser(data)
    })
}
```

### Benchmarks & Allocation Profiling (`testing.B`)
Measure throughput and allocations:

```go
func BenchmarkHashToken(b *testing.B) {
    b.ReportAllocs() // Reports B/op and allocs/op
    token := "tok_super_secure_sample_key"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = HashToken(token)
    }
}
```

### Golden File Testing
For complex serialization (JSON, YAML, Markdown, CLI tables), compare against `.golden` files on disk:

```go
var updateGolden = flag.Bool("update", false, "update golden files")

func TestRenderOutput(t *testing.T) {
    actual := RenderReport(sampleData)
    goldenPath := filepath.Join("testdata", t.Name()+".golden")

    if *updateGolden {
        _ = os.WriteFile(goldenPath, []byte(actual), 0644)
    }

    expected, err := os.ReadFile(goldenPath)
    if err != nil {
        t.Fatalf("missing golden file: %v", err)
    }

    if actual != string(expected) {
        t.Errorf("output mismatch with %s\nDiff:\n%s", goldenPath, diff(actual, string(expected)))
    }
}
```
