package coreconfig

import "testing"

func validTestConfig() CoreAPIConfig {
	return CoreAPIConfig{
		TwitchClientID:      "client",
		TwitchClientSecret:  "secret",
		TwitchRedirectURI:   "https://example.com/api/auth/twitch/callback",
		FrontendBaseURL:     "https://example.com",
		SessionCookieSecure: true,
		SessionTTLMinutes:   480,
	}
}

func TestValidateAcceptsHTTPSProductionConfig(t *testing.T) {
	cfg := validTestConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestValidateAllowsInsecureLoopbackOnly(t *testing.T) {
	cfg := validTestConfig()
	cfg.TwitchRedirectURI = "http://localhost:8080/api/auth/twitch/callback"
	cfg.FrontendBaseURL = "http://localhost:3000"
	cfg.SessionCookieSecure = false
	if err := cfg.Validate(); err != nil {
		t.Fatalf("loopback config rejected: %v", err)
	}

	cfg.TwitchRedirectURI = "http://example.com/api/auth/twitch/callback"
	cfg.FrontendBaseURL = "http://example.com"
	if err := cfg.Validate(); err == nil {
		t.Fatal("insecure non-loopback config was accepted")
	}
}

func TestValidateRejectsWrongCallbackPath(t *testing.T) {
	cfg := validTestConfig()
	cfg.TwitchRedirectURI = "https://example.com/auth/twitch/callback"
	if err := cfg.Validate(); err == nil {
		t.Fatal("wrong callback path was accepted")
	}
}
