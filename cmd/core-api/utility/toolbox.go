package utility

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RejectBadRequest reject request when json body does not validate
func RejectBadRequest(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]string{
		"error": "invalid request body",
	})
}
