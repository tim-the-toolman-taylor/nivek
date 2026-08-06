package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

const (
	SessionCookieName = "nivek_session"
	CSRFCookieName    = "nivek_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
)

type sessionConfiguration interface {
	AuthCookieSecure() bool
	AuthCookieDomain() string
	AuthSessionTTL() time.Duration
}

type cookieOptions struct {
	secure bool
	domain string
	ttl    time.Duration
}

type CookieService struct {
	options cookieOptions
}

func newCookieService(svc nivek.NivekService) *CookieService {
	options := cookieOptions{
		secure: true,
		ttl:    8 * time.Hour,
	}
	if cfg, ok := svc.CustomConfig().(sessionConfiguration); ok {
		options.secure = cfg.AuthCookieSecure()
		options.domain = cfg.AuthCookieDomain()
		if ttl := cfg.AuthSessionTTL(); ttl > 0 {
			options.ttl = ttl
		}
	}
	return &CookieService{options: options}
}

func (s *CookieService) setSessionCookies(c echo.Context, signedToken string) error {
	csrfToken, err := randomToken(32)
	if err != nil {
		return fmt.Errorf("generate csrf token: %w", err)
	}

	expires := time.Now().UTC().Add(s.options.ttl)
	maxAge := int(s.options.ttl.Seconds())

	c.SetCookie(&http.Cookie{
		Name:     SessionCookieName,
		Value:    signedToken,
		Path:     "/api",
		Domain:   s.options.domain,
		Expires:  expires,
		MaxAge:   maxAge,
		Secure:   s.options.secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.SetCookie(&http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		Domain:   s.options.domain,
		Expires:  expires,
		MaxAge:   maxAge,
		Secure:   s.options.secure,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *CookieService) clearSessionCookies(c echo.Context) {
	for _, cookie := range []*http.Cookie{
		{
			Name:     SessionCookieName,
			Path:     "/api",
			Domain:   s.options.domain,
			Secure:   s.options.secure,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		},
		{
			Name:     CSRFCookieName,
			Path:     "/",
			Domain:   s.options.domain,
			Secure:   s.options.secure,
			HttpOnly: false,
			SameSite: http.SameSiteLaxMode,
		},
	} {
		cookie.Value = ""
		cookie.Expires = time.Unix(1, 0).UTC()
		cookie.MaxAge = -1
		c.SetCookie(cookie)
	}
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
