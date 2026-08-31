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

func TestValidateOverlayDisabledWhenBothEmpty(t *testing.T) {
	cfg := validTestConfig() // overlay fields empty -> relay disabled
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty overlay config rejected: %v", err)
	}
}

func TestValidateOverlayAcceptsBothSet(t *testing.T) {
	cfg := validTestConfig()
	cfg.OverlayEventSubSecret = "overlay-secret"
	cfg.OverlayEventSubCallbackURL = "https://peanutbudderbot.com/api/overlay/eventsub"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid overlay config rejected: %v", err)
	}
}

func TestValidateOverlayRejectsPartial(t *testing.T) {
	// Secret without callback: an empty callback would silently point overlay
	// subs at the bot's webhook.
	cfg := validTestConfig()
	cfg.OverlayEventSubSecret = "overlay-secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("overlay secret without callback was accepted")
	}

	// Callback without secret.
	cfg = validTestConfig()
	cfg.OverlayEventSubCallbackURL = "https://peanutbudderbot.com/api/overlay/eventsub"
	if err := cfg.Validate(); err == nil {
		t.Fatal("overlay callback without secret was accepted")
	}
}

func TestValidateOverlayRejectsBadCallback(t *testing.T) {
	// Wrong path.
	cfg := validTestConfig()
	cfg.OverlayEventSubSecret = "overlay-secret"
	cfg.OverlayEventSubCallbackURL = "https://peanutbudderbot.com/eventsub"
	if err := cfg.Validate(); err == nil {
		t.Fatal("overlay callback with wrong path was accepted")
	}

	// Insecure non-loopback.
	cfg = validTestConfig()
	cfg.OverlayEventSubSecret = "overlay-secret"
	cfg.OverlayEventSubCallbackURL = "http://peanutbudderbot.com/api/overlay/eventsub"
	if err := cfg.Validate(); err == nil {
		t.Fatal("overlay callback over http (non-loopback) was accepted")
	}
}

func TestValidateExtensionDisabledWhenBothEmpty(t *testing.T) {
	cfg := validTestConfig() // extension fields empty -> disabled
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty extension config rejected: %v", err)
	}
	if cfg.ExtensionEnabled() {
		t.Fatal("ExtensionEnabled true with no config")
	}
}

func TestValidateExtensionAcceptsFullConfig(t *testing.T) {
	cfg := validTestConfig()
	cfg.OverlayExtensionClientID = "abcdef123456"
	cfg.OverlayExtensionSecret = "c2VjcmV0LWtleS0zMg==" // base64
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid extension config rejected: %v", err)
	}
	if !cfg.ExtensionEnabled() {
		t.Fatal("ExtensionEnabled false with full config")
	}
	if got := cfg.ExtensionAllowedOrigin(); got != "https://abcdef123456.ext-twitch.tv" {
		t.Fatalf("derived origin = %q", got)
	}
}

func TestValidateExtensionRejectsPartial(t *testing.T) {
	cfg := validTestConfig()
	cfg.OverlayExtensionClientID = "abcdef123456"
	if err := cfg.Validate(); err == nil {
		t.Fatal("extension client id without secret was accepted")
	}

	cfg = validTestConfig()
	cfg.OverlayExtensionSecret = "c2VjcmV0"
	if err := cfg.Validate(); err == nil {
		t.Fatal("extension secret without client id was accepted")
	}
}

func TestValidateExtensionRejectsNonBase64Secret(t *testing.T) {
	cfg := validTestConfig()
	cfg.OverlayExtensionClientID = "abcdef123456"
	cfg.OverlayExtensionSecret = "not valid base64 !!!"
	if err := cfg.Validate(); err == nil {
		t.Fatal("non-base64 extension secret was accepted")
	}
}

func TestExtensionAllowedOriginOverride(t *testing.T) {
	cfg := validTestConfig()
	cfg.OverlayExtensionClientID = "abcdef123456"
	cfg.OverlayExtensionSecret = "c2VjcmV0"
	cfg.OverlayExtensionOrigin = "https://localhost:8080/"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("override origin rejected: %v", err)
	}
	if got := cfg.ExtensionAllowedOrigin(); got != "https://localhost:8080" {
		t.Fatalf("override origin = %q", got)
	}
}
