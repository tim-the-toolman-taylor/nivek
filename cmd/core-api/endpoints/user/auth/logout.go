package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type LogoutRequest struct {
	Email string `json:"email"`
}

func NewLogoutEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var logoutRequest LogoutRequest
		if err := c.Bind(&logoutRequest); err != nil {
			return utility.RejectBadRequest(c)
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message": "test",
		})
	}
}
