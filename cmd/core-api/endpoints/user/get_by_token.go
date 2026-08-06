package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// NewGetProfileEndpoint returns the user already resolved by JWT middleware.
// It deliberately does not parse the token a second time.
func NewGetProfileEndpoint(_ nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		userData, ok := c.Get("user").(*userlib.User)
		if !ok || userData == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authenticated user")
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.JSON(http.StatusOK, userData)
	}
}
