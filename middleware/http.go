// Package middleware provides net/http, gin, and echo adapters.
package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	coresdk "github.com/coresdk/go-sdk"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// CoreSDKMiddleware wraps an http.Handler with auth enforcement and OTel span creation.
// Alias for NetHTTP for API compatibility.
func CoreSDKMiddleware(sdk *coresdk.SDK, excludePaths ...string) func(http.Handler) http.Handler {
	return NetHTTP(sdk, excludePaths...)
}

// RequireAuth validates a Bearer token, injects user context, and returns a 401
// with an RFC 9457 body on failure. Unlike NetHTTP, DevMode does not bypass auth.
func RequireAuth(sdk *coresdk.SDK) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearer(r.Header.Get("Authorization"))
			if token == "" {
				writeProblem(w, 401, "Unauthorized", "Missing Authorization header")
				return
			}
			claims, err := sdk.Authorize(r.Context(), token)
			if err != nil {
				writeProblem(w, 401, "Unauthorized", err.Error())
				return
			}
			ctx := coresdk.WithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// NetHTTP returns a net/http middleware that validates JWT and propagates W3C trace context.
func NetHTTP(sdk *coresdk.SDK, excludePaths ...string) func(http.Handler) http.Handler {
	excluded := make(map[string]bool)
	for _, p := range excludePaths {
		excluded[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// W3C trace context propagation
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			r = r.WithContext(ctx)

			if excluded[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			token := extractBearer(r.Header.Get("Authorization"))
			if token == "" && !sdk.Config.DevMode {
				writeProblem(w, 401, "Unauthorized", "Missing Authorization header")
				return
			}

			if token != "" {
				claims, err := sdk.Authorize(r.Context(), token)
				if err != nil {
					if sdk.Config.FailMode == "open" {
						next.ServeHTTP(w, r)
						return
					}
					writeProblem(w, 401, "Unauthorized", err.Error())
					return
				}
				ctx = coresdk.WithClaims(ctx, claims)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractBearer(header string) string {
	if strings.HasPrefix(header, "Bearer ") {
		return header[7:]
	}
	return ""
}

// writeProblem writes an RFC 9457 problem JSON response.
// status is always 401 at current call sites; the parameter is retained for future use.
//
//nolint:unparam // status is intentionally kept as a parameter for extensibility
func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://coresdk.io/errors/unauthorized",
		"title":  title,
		"status": status,
		"detail": detail,
	}); err != nil {
		slog.Error("coresdk: failed to write problem detail", "error", err)
	}
}
