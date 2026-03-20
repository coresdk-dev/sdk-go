package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	coresdk "github.com/coresdk-dev/sdk-go"
	"github.com/coresdk-dev/sdk-go/middleware"
)

func newEchoRouter(sdk *coresdk.SDK) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Echo(sdk))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	return e
}

func TestEcho(t *testing.T) {
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
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := newSDK(tt.failMode, tt.devMode)
			router := newEchoRouter(sdk)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
