package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type CookieService struct {
}

func newCookieService() *CookieService {
	return &CookieService{}
}

// setSessionCookie Set session cookie with secure random ID
func (s *CookieService) setSessionCookie(c echo.Context) error {
	sessionID, err := s.generateSecureSessionID()
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(2 * time.Hour), // Session expires in 2 hours
		MaxAge:   7200,                          // 2 hours in seconds
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	c.SetCookie(cookie)
	return nil
}

func (s *CookieService) generateSecureSessionID() (string, error) {
	bytes := make([]byte, 32) // 256-bit session ID
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
