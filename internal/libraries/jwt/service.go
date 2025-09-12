package jwt

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userlib "github.com/suuuth/nivek/internal/libraries/user"
)

type Service struct {
	nivek         nivek.NivekService
	tokenService  *TokenService
	cookieService *CookieService
}

func NewJWTService(nivek nivek.NivekService) *Service {
	return &Service{
		nivek:         nivek,
		tokenService:  newTokenService(nivek),
		cookieService: newCookieService(),
	}
}

func (s *Service) NewSession(ctx echo.Context, user *userlib.User) (string, error) {
	token, err := s.tokenService.buildToken(user.Id)
	if err != nil {
		return "", fmt.Errorf("error building token: %s", err.Error())
	}

	if err = s.cookieService.setSessionCookie(ctx); err != nil {
		return "", fmt.Errorf("error setting secure session cookie: %s", err.Error())
	}

	return token, nil
}

func (s *Service) ValidateSession(token string) error {
	return s.tokenService.validateToken(token)
}

func (s *Service) GetUserData(token string) (*userlib.User, error) {
	userId, err := s.tokenService.GetUserId(token)
	if err != nil {
		return nil, fmt.Errorf("error getting user data: %w", err)
	}

	userService := userlib.NewService(s.nivek)
	user, err := userService.GetUserById(userId)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	return user, nil
}
