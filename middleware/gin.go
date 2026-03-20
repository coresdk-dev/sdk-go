package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	coresdk "github.com/coresdk-dev/sdk-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// GinMiddleware wraps a gin route group with CoreSDK auth. Alias for Gin.
func GinMiddleware(sdk *coresdk.SDK) gin.HandlerFunc {
	return Gin(sdk)
}

// GinRequireAuth is a strict auth middleware for Gin — DevMode does not bypass auth.
func GinRequireAuth(sdk *coresdk.SDK) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"type":   "https://coresdk.io/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Missing Authorization header",
			})
			return
		}
		claims, err := sdk.Authorize(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"type":   "https://coresdk.io/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": err.Error(),
			})
			return
		}
		c.Set("coresdk_claims", claims)
		c.Next()
	}
}

// Gin returns a gin middleware for CoreSDK auth.
func Gin(sdk *coresdk.SDK) gin.HandlerFunc {
	return func(c *gin.Context) {
		// W3C trace context propagation
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header))
		c.Request = c.Request.WithContext(ctx)

		token := extractBearer(c.GetHeader("Authorization"))
		if token == "" {
			if sdk.Config.DevMode {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"type":   "https://coresdk.io/errors/unauthorized",
				"title":  "Unauthorized",
				"status": 401,
				"detail": "Missing Authorization header",
			})
			return
		}

		claims, err := sdk.Authorize(c.Request.Context(), token)
		if err != nil {
			if sdk.Config.FailMode == "open" {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"title":  "Unauthorized",
				"status": 401,
				"detail": err.Error(),
			})
			return
		}

		c.Set("coresdk_claims", claims)
		c.Next()
	}
}
