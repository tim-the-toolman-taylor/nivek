package jwt

import (
	"fmt"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userlib "github.com/suuuth/nivek/internal/libraries/user"
)

type NivekClaims struct {
	UserId   int    `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`

	jwtlib.RegisteredClaims
}

func (s *TokenService) getClaims(tokenString string) (*NivekClaims, error) {
	token, err := jwtlib.ParseWithClaims(
		tokenString,
		&NivekClaims{},
		func(token *jwtlib.Token) (interface{}, error) {
			return []byte(s.secret), nil
		},
	)

	if err != nil {
		logrus.Errorf("error parsing token: %s", err.Error())
	}

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return token.Claims.(*NivekClaims), nil
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
	claims := NivekClaims{
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
	// Get NivekClaims
	claims, err := s.getClaims(tokenString)
	if err != nil {
		return err
	}

	if claims.UserId == 0 {
		return fmt.Errorf("invalid token")
	}

	return nil
}

func (s *TokenService) GetUserData(tokenString string) (*userlib.User, error) {
	// Get NivekClaims
	claims, err := s.getClaims(tokenString)
	if err != nil {
		return nil, err
	}

	fmt.Println("claims: ", claims)
	fmt.Println("id: ", claims.UserId)

	userData := &userlib.User{
		Id:       claims.UserId,
		Username: claims.Username,
		Email:    claims.Email,
		Role:     claims.Role,
	}

	if userData.Id == 0 {
		return nil, fmt.Errorf("invalid token")
	}

	return userData, nil
}
