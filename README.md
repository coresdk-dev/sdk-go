# CoreSDK Go SDK

Go client for [CoreSDK](https://github.com/coresdk-dev/core-sdk) — JWT validation, policy evaluation, and feature flags via the CoreSDK sidecar.

**New here?** The [Getting Started guide](GETTING-STARTED.md) takes you from zero to a working sidecar + SDK call in 15 minutes, with a **why** explanation at every step.

## Install

```bash
go get github.com/coresdk-dev/sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    coresdk "github.com/coresdk-dev/sdk-go"
)

func main() {
    sdk, err := coresdk.FromEnv()
    if err != nil {
        log.Fatal(err)
    }

    // Validate a JWT
    claims, err := sdk.Authorize(context.Background(), jwtToken)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Subject: %s, Roles: %v\n", claims.Subject, claims.Roles)

    // Evaluate a Rego policy
    allowed, err := sdk.EvaluatePolicy(context.Background(), "data.myapp.allow", map[string]any{
        "action": "read",
        "roles":  claims.Roles,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Policy allowed: %v\n", allowed)
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CORESDK_SIDECAR_ADDR` | `localhost:50051` | Sidecar gRPC address |
| `CORESDK_TENANT_ID` | `default` | Tenant identifier |
| `CORESDK_FAIL_MODE` | `open` | `open` = allow on error, `closed` = deny |
| `CORESDK_ENV` | `production` | Set to `development` for dev mode |
| `CORESDK_SERVICE_NAME` | `unknown-service` | Service name for tracing |
| `CORESDK_TLS_CERT` | — | Client certificate path (mTLS) |
| `CORESDK_TLS_KEY` | — | Client key path (mTLS) |
| `CORESDK_TLS_CA` | — | CA certificate path (mTLS) |

## mTLS

To enable mTLS between your app and the sidecar, set all three TLS environment variables:

```bash
export CORESDK_TLS_CERT=/path/to/client.crt
export CORESDK_TLS_KEY=/path/to/client.key
export CORESDK_TLS_CA=/path/to/ca.crt
```

The SDK automatically configures TLS 1.3 mutual authentication when these are present.

## Framework Middleware

### Gin

```go
import "github.com/coresdk-dev/sdk-go/middleware"

r := gin.Default()
r.Use(middleware.Gin(sdk))

// Or strict mode (no dev bypass):
r.Use(middleware.GinRequireAuth(sdk))
```

### Echo

```go
import "github.com/coresdk-dev/sdk-go/middleware"

e := echo.New()
e.Use(middleware.Echo(sdk))
```

### net/http

```go
import "github.com/coresdk-dev/sdk-go/middleware"

mux := http.NewServeMux()
handler := middleware.NetHTTP(sdk, "/health")(mux)

// Or strict mode:
handler := middleware.RequireAuth(sdk)(mux)
```

All middleware propagates W3C trace context and stores claims in the request context. Retrieve claims downstream:

```go
claims, ok := coresdk.ClaimsFrom(r.Context())
```

## Testing

Use `MockSDK` as a drop-in test double:

```go
import coresdk "github.com/coresdk-dev/sdk-go"

func TestMyHandler(t *testing.T) {
    // Allow all by default
    sdk := coresdk.NewMockSDK(coresdk.MockConfig{})

    claims, err := sdk.Authorize(context.Background(), "any-token")
    // claims.Subject == "test-user", claims.Roles == ["member"]

    // Deny all
    sdk = coresdk.NewMockSDK(coresdk.MockConfig{DenyAll: true})
    _, err = sdk.Authorize(context.Background(), "any-token")
    // err != nil

    // Assert calls were made
    fmt.Println(sdk.AuthorizeCalls) // ["any-token"]
}
```

## Development

```bash
# Run tests (no sidecar needed for unit tests)
make test-unit

# Full check (vet + test)
make check

# Install golangci-lint and run linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
make lint
```

## Sidecar

```bash
docker run --rm \
  -e CORESDK_ENV=development \
  -e CORESDK_SIDECAR_ADDR=[::]:50051 \
  -p 50051:50051 \
  -p 9091:9091 \
  ghcr.io/coresdk-dev/sidecar:latest
# Verify: curl http://localhost:9091/healthz  →  {"status":"ok"}
```

See the [Getting Started guide](GETTING-STARTED.md) for the full setup walkthrough.

## Jobs (containerised long-running work)

`Client` exposes the sidecar's `JobService` for async K8s-backed
container workloads with typed lifecycle events, blob-store I/O, and
RBAC-gated secret injection.

```go
sdk, _ := coresdk.FromEnv()

job, _ := sdk.SubmitJob(ctx, coresdk.SubmitJobRequest{
    Kind:           "claude-cli",
    Image:          "ghcr.io/zysec/cpod-claude-cli:latest",
    Command:        []string{"claude"},
    InlineFiles:    map[string][]byte{"prompt.md": []byte("hi")},
    SecretBundles:  []string{"anthropic-prod"},
    UserID:         "alice@example.com",
    TimeoutSeconds: 600,
})

evs, _ := sdk.WatchJob(ctx, job.JobID)
for ev := range evs {
    switch ev.Kind {
    case coresdk.JobEventProgress:
        log.Println(ev.Stage, ev.Detail)
    case coresdk.JobEventSucceeded:
        out, _ := sdk.GetJobOutput(ctx, job.JobID, 900)
        for _, f := range out.Files {
            log.Println(f.Key, f.PresignedURL)
        }
    case coresdk.JobEventFailed:
        log.Fatal(ev.Error)
    }
}
```

`WatchJob` and `StreamJobLogs` return typed Go channels; closing the
context cancels the underlying gRPC stream. Public types: `Job`,
`JobEvent` with `JobEventKind` discriminator, `LogLine`, `OutputFile`,
`JobOutput`, `SecretRef`, `SubmitJobRequest`.

## License

See [LICENSE](../LICENSE).
