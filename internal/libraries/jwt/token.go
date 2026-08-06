package jwt

import (
	"fmt"
	"os"
	"strconv"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	minJWTSecretBytes = 32
	tokenIssuer       = "nivek-core-api"
	tokenAudience     = "nivek-web"
)

type NivekClaims struct {
	UserID int `json:"user_id"`
	jwtlib.RegisteredClaims
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func newTokenService(ttl time.Duration) *TokenService {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &TokenService{
		secret: []byte(os.Getenv("JWT_SECRET")),
		ttl:    ttl,
		now:    time.Now,
	}
}

func (s *TokenService) getClaims(tokenString string) (*NivekClaims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("missing token")
	}

	claims := &NivekClaims{}
	token, err := jwtlib.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwtlib.Token) (any, error) {
			if token.Method != jwtlib.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return s.secret, nil
		},
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
		jwtlib.WithIssuer(tokenIssuer),
		jwtlib.WithAudience(tokenAudience),
		jwtlib.WithExpirationRequired(),
		jwtlib.WithIssuedAt(),
		jwtlib.WithLeeway(30*time.Second),
	)
	if err != nil || token == nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.UserID <= 0 || claims.Subject != strconv.Itoa(claims.UserID) || claims.ID == "" {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func ValidateJWTSecret() error {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minJWTSecretBytes {
		return fmt.Errorf("JWT_SECRET must be at least %d bytes (got %d)", minJWTSecretBytes, len(secret))
	}
	return nil
}

func (s *TokenService) buildToken(userID int) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("invalid user id")
	}
	if len(s.secret) < minJWTSecretBytes {
		return "", fmt.Errorf("JWT secret is not configured securely")
	}

	now := s.now().UTC()
	claims := NivekClaims{
		UserID: userID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   strconv.Itoa(userID),
			Audience:  jwtlib.ClaimStrings{tokenAudience},
			ExpiresAt: jwtlib.NewNumericDate(now.Add(s.ttl)),
			NotBefore: jwtlib.NewNumericDate(now.Add(-30 * time.Second)),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *TokenService) GetUserID(tokenString string) (int, error) {
	claims, err := s.getClaims(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}

// GetUserId remains as a compatibility alias.
func (s *TokenService) GetUserId(tokenString string) (int, error) {
	return s.GetUserID(tokenString)
}
