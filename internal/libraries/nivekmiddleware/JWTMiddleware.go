package nivekmiddleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

type JWTMiddleware interface {
	Middleware() echo.MiddlewareFunc
}

type jwtMiddlewareImpl struct {
	nivek      nivek.NivekService
	jwtService *jwt.Service
}

type credentialSource string

const (
	credentialSourceBearer credentialSource = "bearer"
	credentialSourceCookie credentialSource = "cookie"
)

func NewJWTMiddleware(svc nivek.NivekService) JWTMiddleware {
	return &jwtMiddlewareImpl{
		nivek:      svc,
		jwtService: jwt.NewJWTService(svc),
	}
}

func (m *jwtMiddlewareImpl) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tokenString, source, err := extractCredential(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}

			if source == credentialSourceCookie && requestNeedsCSRF(c.Request().Method) {
				if err := validateCSRF(c.Request()); err != nil {
					return echo.NewHTTPError(http.StatusForbidden, err.Error())
				}
			}

			user, err := m.jwtService.GetUserData(tokenString)
			if err != nil {
				m.nivek.Logger().Debugf("session validation failed: %s", err.Error())
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired session")
			}

			c.Set("user", user)
			c.Set("auth_source", string(source))
			return next(c)
		}
	}
}

func extractCredential(request *http.Request) (string, credentialSource, error) {
	authorization := strings.TrimSpace(request.Header.Get(echo.HeaderAuthorization))
	if authorization != "" {
		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
			return "", "", fmt.Errorf("malformed authorization header")
		}
		return parts[1], credentialSourceBearer, nil
	}

	cookie, err := request.Cookie(jwt.SessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", "", fmt.Errorf("missing session")
	}
	return cookie.Value, credentialSourceCookie, nil
}

func requestNeedsCSRF(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func validateCSRF(request *http.Request) error {
	cookie, err := request.Cookie(jwt.CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return fmt.Errorf("missing csrf cookie")
	}
	header := request.Header.Get(jwt.CSRFHeaderName)
	if header == "" || len(header) != len(cookie.Value) {
		return fmt.Errorf("invalid csrf token")
	}
	if subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) != 1 {
		return fmt.Errorf("invalid csrf token")
	}
	return nil
}
