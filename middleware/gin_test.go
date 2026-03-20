package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	coresdk "github.com/coresdk-dev/sdk-go"
	"github.com/coresdk-dev/sdk-go/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newGinRouter(sdk *coresdk.SDK) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Gin(sdk))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestGin(t *testing.T) {
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
			router := newGinRouter(sdk)

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

func TestGinRequireAuth(t *testing.T) {
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
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdk := newSDK(tt.failMode, false)
			r := gin.New()
			r.Use(middleware.GinRequireAuth(sdk))
			r.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
