package user

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

func NewGetProfileEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		tokenString := strings.TrimPrefix(
			c.Request().Header.Get("Authorization"),
			"Bearer ",
		)

		jwtService := jwt.NewJWTService(nivek)
		userData, err := jwtService.GetUserData(tokenString)
		if err != nil {
			logrus.Errorf("failed to get user data from token string: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to get user data",
			})
		}

		return c.JSON(http.StatusOK, *userData)
	}
}
