package nivekmiddleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type JWTMiddleware interface {
	Middleware() echo.MiddlewareFunc
}

type jwtMiddlewareImpl struct {
	nivek      nivek.NivekService
	jwtService *jwt.Service
}

func NewJWTMiddleware(nivek nivek.NivekService) JWTMiddleware {
	return &jwtMiddlewareImpl{
		nivek:      nivek,
		jwtService: jwt.NewJWTService(nivek),
	}
}

func (m *jwtMiddlewareImpl) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenString := strings.TrimPrefix(
				c.Request().Header.Get("Authorization"),
				"Bearer ",
			)
			if tokenString == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			if err := m.jwtService.ValidateSession(tokenString); err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			user, err := m.jwtService.GetUserData(tokenString)
			if err != nil {
				logrus.Errorf("failed to get user data from token string: %s", err.Error())
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "failed to get user data",
				})
			}

			c.Set("user", user)

			return next(c)
		}
	}
}
