package nivekmiddleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/jwt"
)

func TestExtractCredentialStrictBearer(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.Header.Set("Authorization", "Bearer abc.def.ghi")
	token, source, err := extractCredential(req)
	if err != nil || token != "abc.def.ghi" || source != credentialSourceBearer {
		t.Fatalf("unexpected result token=%q source=%q err=%v", token, source, err)
	}

	req.Header.Set("Authorization", "Bearer")
	if _, _, err := extractCredential(req); err == nil {
		t.Fatal("expected malformed bearer header to fail")
	}
}

func TestExtractCredentialFromCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	req.AddCookie(&http.Cookie{Name: jwt.SessionCookieName, Value: "signed-token"})
	token, source, err := extractCredential(req)
	if err != nil || token != "signed-token" || source != credentialSourceCookie {
		t.Fatalf("unexpected result token=%q source=%q err=%v", token, source, err)
	}
}

func TestValidateCSRF(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/weather", nil)
	req.AddCookie(&http.Cookie{Name: jwt.CSRFCookieName, Value: "csrf-value"})
	req.Header.Set(jwt.CSRFHeaderName, "csrf-value")
	if err := validateCSRF(req); err != nil {
		t.Fatalf("valid csrf rejected: %v", err)
	}

	req.Header.Set(jwt.CSRFHeaderName, "wrong-value")
	if err := validateCSRF(req); err == nil {
		t.Fatal("mismatched csrf token was accepted")
	}
}
