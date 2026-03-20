package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	coresdk "github.com/coresdk-dev/sdk-go"
	"github.com/coresdk-dev/sdk-go/middleware"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck
}

func newSDK(failMode string, devMode bool) *coresdk.SDK {
	return &coresdk.SDK{
		Config: &coresdk.Config{
			FailMode: failMode,
			DevMode:  devMode,
		},
	}
}

func TestNetHTTP(t *testing.T) {
	tests := []struct {
		name       string
		failMode   string
		devMode    bool
		authHeader string
		wantStatus int
	}{
		{
			name:       "missing auth header returns 401",
			failMode:   "open",
			devMode:    false,
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "dev mode bypasses missing auth",
			failMode:   "open",
			devMode:    true,
			authHeader: "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid token fail-open passes through",
			failMode:   "open",
			devMode:    false,
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusOK, // nil client → Authorize fail-open
		},
		{
			name:       "invalid token fail-closed returns 401",
			failMode:   "closed",
			devMode:    false,
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusUnauthorized, // nil client → Authorize returns error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := newSDK(tt.failMode, tt.devMode)
			handler := middleware.NetHTTP(sdk)(http.HandlerFunc(okHandler))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name       string
		failMode   string
		authHeader string
		wantStatus int
	}{
		{
			name:       "missing auth header returns 401",
			failMode:   "open",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token fail-closed returns 401",
			failMode:   "closed",
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token fail-open passes through",
			failMode:   "open",
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusOK, // nil client → Authorize fail-open → claims returned → 200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := newSDK(tt.failMode, false)
			handler := middleware.RequireAuth(sdk)(http.HandlerFunc(okHandler))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
