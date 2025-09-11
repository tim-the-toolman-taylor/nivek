package nivekmiddleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
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

			fmt.Println("validated by middleware authentication for route: ", c.Path())

			return nil
		}
	}
}
