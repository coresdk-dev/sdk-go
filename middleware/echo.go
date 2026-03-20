package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	coresdk "github.com/coresdk-dev/sdk-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Echo returns an echo middleware for CoreSDK auth.
func Echo(sdk *coresdk.SDK) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// W3C trace context propagation
			ctx := otel.GetTextMapPropagator().Extract(c.Request().Context(),
				propagation.HeaderCarrier(c.Request().Header))
			c.SetRequest(c.Request().WithContext(ctx))

			token := extractBearer(c.Request().Header.Get("Authorization"))
			if token == "" {
				if sdk.Config.DevMode {
					return next(c)
				}
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"type":   "https://coresdk.io/errors/unauthorized",
					"title":  "Unauthorized",
					"status": 401,
					"detail": "Missing Authorization header",
				})
			}

			claims, err := sdk.Authorize(c.Request().Context(), token)
			if err != nil {
				if sdk.Config.FailMode == "open" {
					return next(c)
				}
				return c.JSON(http.StatusUnauthorized, map[string]any{
					"title":  "Unauthorized",
					"status": 401,
					"detail": err.Error(),
				})
			}

			c.Set("coresdk_claims", claims)
			return next(c)
		}
	}
}
