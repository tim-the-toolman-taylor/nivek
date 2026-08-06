package jwt

import (
	"strconv"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func testTokenService() *TokenService {
	return &TokenService{
		secret: []byte("0123456789abcdef0123456789abcdef"),
		ttl:    time.Hour,
		now:    func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}
}

func TestTokenRoundTrip(t *testing.T) {
	svc := testTokenService()
	svc.now = time.Now
	token, err := svc.buildToken(42)
	if err != nil {
		t.Fatalf("buildToken: %v", err)
	}
	claims, err := svc.getClaims(token)
	if err != nil {
		t.Fatalf("getClaims: %v", err)
	}
	if claims.UserID != 42 || claims.Subject != "42" || claims.Issuer != tokenIssuer {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRejectsWrongSigningMethod(t *testing.T) {
	svc := testTokenService()
	now := time.Now().UTC()
	claims := NivekClaims{
		UserID: 7,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   strconv.Itoa(7),
			Audience:  jwtlib.ClaimStrings{tokenAudience},
			ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ID:        "test-id",
		},
	}
	bad := jwtlib.NewWithClaims(jwtlib.SigningMethodHS384, claims)
	signed, err := bad.SignedString(svc.secret)
	if err != nil {
		t.Fatalf("sign bad token: %v", err)
	}
	if _, err := svc.getClaims(signed); err == nil {
		t.Fatal("expected HS384 token to be rejected")
	}
}

func TestRejectsWrongIssuerAndAudience(t *testing.T) {
	svc := testTokenService()
	now := time.Now().UTC()
	claims := NivekClaims{
		UserID: 9,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    "other-service",
			Subject:   "9",
			Audience:  jwtlib.ClaimStrings{"other-client"},
			ExpiresAt: jwtlib.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ID:        "test-id",
		},
	}
	bad := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := bad.SignedString(svc.secret)
	if err != nil {
		t.Fatalf("sign bad token: %v", err)
	}
	if _, err := svc.getClaims(signed); err == nil {
		t.Fatal("expected wrong issuer/audience token to be rejected")
	}
}
