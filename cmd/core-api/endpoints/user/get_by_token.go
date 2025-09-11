package user

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

func NewGetProfileEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		logrus.Infof("get profile data by token")

		tokenString := strings.TrimPrefix(
			c.Request().Header.Get("Authorization"),
			"Bearer ",
		)
		if tokenString == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
		}

		jwtService := jwt.NewJWTService(nivek)
		if err := jwtService.ValidateSession(tokenString); err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}

		fmt.Println("here")

		userData, err := jwtService.GetUserData(tokenString)
		if err != nil {
			logrus.Errorf("failed to get user data from token string: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to get user data",
			})
		}

		fmt.Println("userdata: ", userData)

		return c.JSON(http.StatusOK, *userData)
	}
}
