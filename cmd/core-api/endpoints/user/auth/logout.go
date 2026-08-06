package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

// NewLogoutEndpoint clears the stateless session and CSRF cookies. The route is
// intentionally usable even when the JWT has expired so clients can always
// leave a clean browser state.
func NewLogoutEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		jwt.NewJWTService(svc).ClearSession(c)
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.NoContent(http.StatusNoContent)
	}
}
