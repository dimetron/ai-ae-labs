# Go Security Manual: Senior Developer Standards

This guide covers security best practices for developing resilient Go backends and microservices, defending against common vulnerability vectors (OWASP Top 10).

---

## 1. Preventing SQL Injection

Always use parameterized placeholders provided by the driver. **Never** concatenate user input into raw query strings.

### Correct Parameterization
```go
// ✅ Safe: driver safely escapes parameters
query := `SELECT id, name, email FROM users WHERE organization_id = $1 AND status = $2`
rows, err := db.QueryContext(ctx, query, orgID, status)
```

### Vulnerable Concatenation (Anti-Pattern)
```go
// ❌ CRITICAL VULNERABILITY: SQL Injection
query := fmt.Sprintf("SELECT id FROM users WHERE username = '%s'", userInput)
```

### Safe Dynamic Queries
When building dynamic search queries with optional filters, parameterize each filter and use a slice of arguments:

```go
var conditions []string
var args []any
argID := 1

if status != "" {
    conditions = append(conditions, fmt.Sprintf("status = $%d", argID))
    args = append(args, status)
    argID++
}

query := "SELECT id FROM orders WHERE " + strings.Join(conditions, " AND ")
rows, err := db.QueryContext(ctx, query, args...)
```

---

## 2. SSRF (Server-Side Request Forgery) Prevention

When your service fetches resources from URLs supplied by users, malicious actors may target internal services (`169.254.169.254`, `127.0.0.1`, or RFC 1918 private subnets).

### Safe HTTP Client Validation

```go
package netsec

import (
    "context"
    "errors"
    "net"
    "net/http"
    "net/url"
    "time"
)

// SafeClient returns an http.Client that refuses connections to private or loopback IPs
func SafeClient() *http.Client {
    dialer := &net.Dialer{
        Timeout: 5 * time.Second,
    }

    transport := &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            host, port, err := net.SplitHostPort(addr)
            if err != nil {
                return nil, err
            }

            ips, err := net.LookupIP(host)
            if err != nil {
                return nil, err
            }

            for _, ip := range ips {
                if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
                    return nil, errors.New("access to private network address is forbidden")
                }
            }

            return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
        },
    }

    return &http.Client{
        Transport: transport,
        Timeout:   10 * time.Second,
    }
}
```

---

## 3. Path Traversal Defense

Never pass raw user-supplied filenames directly to `os.Open` or `os.ReadFile`.

```go
package filesec

import (
    "errors"
    "fmt"
    "path/filepath"
    "strings"
)

func SafeFilePath(baseDir, userPath string) (string, error) {
    // 1. Clean the path to resolve ../ and ./
    cleaned := filepath.Clean(userPath)

    // 2. Form absolute path
    fullPath := filepath.Join(baseDir, cleaned)

    // 3. Ensure the target path resides within baseDir
    rel, err := filepath.Rel(baseDir, fullPath)
    if err != nil || strings.HasPrefix(rel, "..") {
        return "", fmt.Errorf("path traversal attempt detected: %q", userPath)
    }

    return fullPath, nil
}
```

---

## 4. Cryptography & Secrets Hygiene

### Cryptographically Secure Randomness
Never use `math/rand` for security tokens, passwords, or session IDs. Always use `crypto/rand`:

```go
import (
    "crypto/rand"
    "encoding/hex"
)

func GenerateSecureToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", fmt.Errorf("reading cryptographic random bytes: %w", err)
    }
    return hex.EncodeToString(bytes), nil
}
```

### Password Hashing
Always use `bcrypt` or `argon2id`:

```go
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(bytes), err
}

func CheckPassword(password, hash string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

### Preventing Secret Leaks in Logs & Structs
Redact sensitive types by implementing custom formatting:

```go
type SecretString string

func (s SecretString) String() string {
    return "[REDACTED]"
}

func (s SecretString) MarshalJSON() ([]byte, error) {
    return []byte(`"[REDACTED]"`), nil
}
```

---

## 5. Automated Security Tooling

### `govulncheck` (Official Go Vulnerability Scanner)
Include `govulncheck` in your CI/CD pipelines to scan dependencies against the Go vulnerability database:

```bash
# Install tool
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run scan
govulncheck ./...
```

### `gosec` Static Security Analysis
Use `gosec` to detect hardcoded credentials, unhandled errors, weak TLS configurations, and unsafe block usage:

```bash
go run github.com/securego/gosec/v2/cmd/gosec@latest ./...
```
