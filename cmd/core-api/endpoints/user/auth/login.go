package auth

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userLib "github.com/suuuth/nivek/internal/libraries/user"
)

func NewLoginEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var loginRequest userLib.LoginRequest
		if err := c.Bind(&loginRequest); err != nil {
			return utility.RejectBadRequest(c)
		}

		userService := userLib.NewService(nivek)
		user, err := userService.Login(loginRequest)
		if err != nil {
			logrus.Errorf("failed to authenticate user data: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "login failed",
			})
		}

		jwtService := jwt.NewJWTService(nivek)
		token, err := jwtService.NewSession(c, user)
		if err != nil {
			logrus.Errorf("failed to generate session: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "login failed",
			})
		}

		c.Response().Header().Set(echo.HeaderAuthorization, fmt.Sprintf("Bearer %s", token))
		return c.JSON(http.StatusOK, user)
	}
}
