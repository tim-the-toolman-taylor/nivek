package jwt

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

type Service struct {
	nivek         nivek.NivekService
	tokenService  *TokenService
	cookieService *CookieService
}

func NewJWTService(svc nivek.NivekService) *Service {
	cookies := newCookieService(svc)
	return &Service{
		nivek:         svc,
		tokenService:  newTokenService(cookies.options.ttl),
		cookieService: cookies,
	}
}

func (s *Service) NewSession(ctx echo.Context, user *userlib.User) error {
	if user == nil || user.Id <= 0 {
		return fmt.Errorf("cannot create session for an invalid user")
	}

	token, err := s.tokenService.buildToken(user.Id)
	if err != nil {
		return fmt.Errorf("build session token: %w", err)
	}
	if err := s.cookieService.setSessionCookies(ctx, token); err != nil {
		s.cookieService.clearSessionCookies(ctx)
		return fmt.Errorf("set session cookies: %w", err)
	}
	return nil
}

func (s *Service) ClearSession(ctx echo.Context) {
	s.cookieService.clearSessionCookies(ctx)
}

func (s *Service) ValidateSession(token string) error {
	_, err := s.tokenService.getClaims(token)
	return err
}

func (s *Service) GetUserData(token string) (*userlib.User, error) {
	userID, err := s.tokenService.GetUserID(token)
	if err != nil {
		return nil, fmt.Errorf("get user id: %w", err)
	}

	userService := userlib.NewService(s.nivek)
	user, err := userService.GetUserById(userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}
