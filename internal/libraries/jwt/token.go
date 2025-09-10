package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type Claims struct {
	UserId   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`

	jwtlib.RegisteredClaims
}

type TokenService struct {
	secret string
}

func newTokenService(nivek nivek.NivekService) *TokenService {
	return &TokenService{
		secret: nivek.CommonConfig().AppName, // @TODO::replace this with a dedicated var,
	}
}

func (s *TokenService) buildToken(
	userID int,
	username,
	email,
	role string,
) (
	string,
	error,
) {
	// Create the claims
	claims := Claims{
		UserId:   userID,
		Username: username,
		Email:    email,
		Role:     role,

		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour * 24)), // Expires in 24 hours
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),                     // Issued at
		},
	}

	// Create token with claims
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)

	// Generate encoded token
	tokenString, err := token.SignedString([]byte(s.secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (s *TokenService) validateToken(tokenString string) error {
	token, err := jwtlib.ParseWithClaims(
		tokenString,
		&jwtlib.MapClaims{},
		func(token *jwtlib.Token) (interface{}, error) {
			return []byte(s.secret), nil
		},
	)

	if err != nil {
		logrus.Errorf("error parsing token: %s", err.Error())
	}

	if err != nil || !token.Valid {
		return fmt.Errorf("invalid token")
	}

	// Get Claims
	claims := token.Claims.(*Claims)

	if claims.UserId == 0 {
		return fmt.Errorf("invalid token")
	}

	return nil
}
