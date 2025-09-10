package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userLib "github.com/suuuth/nivek/internal/libraries/user"
)

func NewSignupEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var signupRequest userLib.SignupRequest
		if err := c.Bind(&signupRequest); err != nil {
			return utility.RejectBadRequest(c)
		}

		userService := userLib.NewService(nivek)
		success, err := userService.Signup(signupRequest)
		if err != nil {
			logrus.Errorf("failed to login: %s", err.Error())
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "Login failed",
			})
		}

		return c.JSON(http.StatusOK, success)
	}
}
