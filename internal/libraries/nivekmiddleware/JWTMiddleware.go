package nivekmiddleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type JWTMiddleware struct {
	nivek      nivek.NivekService
	jwtService *jwt.Service
}

func NewJWTMiddleware(nivek nivek.NivekService) *JWTMiddleware {
	return &JWTMiddleware{
		nivek:      nivek,
		jwtService: jwt.NewJWTService(nivek),
	}
}

func (m *JWTMiddleware) Run() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			logrus.Infof("running jwt middleware")

			tokenString := strings.TrimPrefix(
				c.Request().Header.Get("Authorization"),
				"Bearer ",
			)
			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			logrus.Infof("jwt token: %s", tokenString)

			if err := m.jwtService.ValidateSession(tokenString); err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			logrus.Infof("verified jwt token: %s", tokenString)

			return nil
		}
	}
}
