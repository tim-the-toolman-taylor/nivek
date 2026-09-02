// Package coreconfig holds the core-api process configuration.
package coreconfig

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/twitchext"
)

var staticCoreAPIConfig *CoreAPIConfig

func GetCoreAPIConfig() CoreAPIConfig {
	if staticCoreAPIConfig == nil {
		parsed := Parse()
		staticCoreAPIConfig = &parsed
	}
	return *staticCoreAPIConfig
}

// GetCoreApiConfig is retained for compatibility with existing call sites.
func GetCoreApiConfig() CoreAPIConfig { return GetCoreAPIConfig() }

func Parse() (config CoreAPIConfig) {
	if err := godotenv.Load(); err != nil {
		// stdlib log on purpose: config parsing runs before the nivek
		// service (and its logger) exists.
		log.Print("No .env file found, using environment variables")
	}
	envconfig.MustProcess("", &config)
	config.FrontendBaseURL = strings.TrimRight(config.FrontendBaseURL, "/")
	return config
}

type CoreAPIConfig struct {
	APIServerPort string `envconfig:"CORE_API_PORT" default:"8080"`
	ListenAddress string `envconfig:"CORE_API_LISTEN_ADDRESS" default:"0.0.0.0"`

	TwitchEventSubSecret      string `envconfig:"TWITCH_EVENTSUB_SECRET" default:""`
	TwitchEventSubCallbackURL string `envconfig:"TWITCH_EVENTSUB_CALLBACK_URL" default:""`
	TwitchClientID            string `envconfig:"TWITCH_CLIENT_ID" default:""`
	TwitchClientSecret        string `envconfig:"TWITCH_CLIENT_SECRET" default:""`
	TwitchRedirectURI         string `envconfig:"TWITCH_REDIRECT_URI" default:""`
	FrontendBaseURL           string `envconfig:"FRONTEND_BASE_URL" default:""`

	SessionCookieSecure bool   `envconfig:"SESSION_COOKIE_SECURE" default:"true"`
	SessionCookieDomain string `envconfig:"SESSION_COOKIE_DOMAIN" default:""`
	SessionTTLMinutes   int    `envconfig:"SESSION_TTL_MINUTES" default:"480"`

	// Overlay relay. A dedicated EventSub secret and callback, independent of
	// the bot's own /eventsub: the two subscription sets are created, revoked,
	// and rotated separately. Empty disables the relay's webhook (503) rather
	// than failing boot, so existing deployments keep starting.
	OverlayEventSubSecret      string `envconfig:"OVERLAY_EVENTSUB_SECRET" default:""`
	OverlayEventSubCallbackURL string `envconfig:"OVERLAY_EVENTSUB_CALLBACK_URL" default:""`

	// Twitch Extension (Bits) ingest. The extension's client id and its shared
	// secret (base64, from the developer console) let the backend verify the
	// signed Bits receipts a viewer's browser posts. Empty disables the
	// extension endpoint (503). OverlayExtensionOrigin overrides the browser
	// origin allowed through CORS; empty derives https://<client id>.ext-twitch.tv
	// (where Hosted/Released extensions run) -- set it to the local-test base URI
	// when testing the panel in a browser.
	OverlayExtensionClientID string `envconfig:"OVERLAY_EXTENSION_CLIENT_ID" default:""`
	OverlayExtensionSecret   string `envconfig:"OVERLAY_EXTENSION_SECRET" default:""`
	OverlayExtensionOrigin   string `envconfig:"OVERLAY_EXTENSION_ORIGIN" default:""`

	// Bot listener, reached over the Docker gateway.
	BotInternalURL string `envconfig:"BOT_INTERNAL_URL" default:"http://172.19.0.1:8090"`
	BotAPIHMACKey  string `envconfig:"BOT_API_HMAC_KEY" default:""`

	// Gated overlay-build download (redistribution POC). OverlayDownloadPath is
	// the build's path inside the container -- mount it read-only via compose
	// rather than baking a ~50MB binary into the image. OverlayDownloadAllowlist
	// is a comma-separated list of Twitch logins allowed to download it. Either
	// one empty disables the endpoint (404), so a deployment without a build
	// configured simply has no download route.
	OverlayDownloadPath      string `envconfig:"OVERLAY_DOWNLOAD_PATH" default:""`
	OverlayDownloadAllowlist string `envconfig:"OVERLAY_DOWNLOAD_ALLOWLIST" default:""`
}

// CoreApiConfig is retained as a type alias for compatibility.
type CoreApiConfig = CoreAPIConfig

func (c CoreAPIConfig) Validate() error {
	var problems []string

	if strings.TrimSpace(c.TwitchClientID) == "" {
		problems = append(problems, "TWITCH_CLIENT_ID is required")
	}
	if strings.TrimSpace(c.TwitchClientSecret) == "" {
		problems = append(problems, "TWITCH_CLIENT_SECRET is required")
	}
	if c.SessionTTLMinutes < 15 || c.SessionTTLMinutes > 1440 {
		problems = append(problems, "SESSION_TTL_MINUTES must be between 15 and 1440")
	}

	redirect, err := validateAbsoluteURL("TWITCH_REDIRECT_URI", c.TwitchRedirectURI)
	if err != nil {
		problems = append(problems, err.Error())
	} else if redirect.Path != "/api/auth/twitch/callback" {
		problems = append(problems, "TWITCH_REDIRECT_URI path must be /api/auth/twitch/callback")
	}

	frontend, err := validateAbsoluteURL("FRONTEND_BASE_URL", c.FrontendBaseURL)
	if err != nil {
		problems = append(problems, err.Error())
	} else if frontend.RawQuery != "" || frontend.Fragment != "" || frontend.User != nil {
		problems = append(problems, "FRONTEND_BASE_URL must not contain credentials, a query, or a fragment")
	}

	if redirect != nil && frontend != nil {
		if !strings.EqualFold(redirect.Hostname(), frontend.Hostname()) {
			problems = append(problems, "TWITCH_REDIRECT_URI and FRONTEND_BASE_URL must use the same hostname so the session cookie reaches the frontend API")
		}

		loopback := isLoopbackHost(redirect.Hostname())
		if c.SessionCookieSecure && redirect.Scheme != "https" && !loopback {
			problems = append(problems, "SESSION_COOKIE_SECURE=true requires an HTTPS Twitch redirect URI outside local development")
		}
		if !c.SessionCookieSecure && !loopback {
			problems = append(problems, "SESSION_COOKIE_SECURE=false is permitted only for localhost or loopback development")
		}
	}

	if c.SessionCookieDomain != "" && frontend != nil {
		domain := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(c.SessionCookieDomain)), ".")
		host := strings.ToLower(frontend.Hostname())
		if strings.ContainsAny(domain, "/:") || (host != domain && !strings.HasSuffix(host, "."+domain)) {
			problems = append(problems, "SESSION_COOKIE_DOMAIN must be the frontend hostname or its parent domain")
		}
	}

	if c.TwitchEventSubCallbackURL != "" {
		callback, callbackErr := validateAbsoluteURL("TWITCH_EVENTSUB_CALLBACK_URL", c.TwitchEventSubCallbackURL)
		if callbackErr != nil {
			problems = append(problems, callbackErr.Error())
		} else if callback.Scheme != "https" && !isLoopbackHost(callback.Hostname()) {
			problems = append(problems, "TWITCH_EVENTSUB_CALLBACK_URL must use HTTPS outside local development")
		}
	}

	// Overlay relay: empty disables it, but a half-configured relay (only one of
	// secret/callback) is a mistake. An empty callback in particular is dangerous
	// -- the twitcheventsub client defaults a blank CallbackURL to the BOT's
	// /eventsub, which would point overlay subscriptions at the wrong webhook.
	if c.OverlayEventSubSecret != "" || c.OverlayEventSubCallbackURL != "" {
		if strings.TrimSpace(c.OverlayEventSubSecret) == "" {
			problems = append(problems, "OVERLAY_EVENTSUB_SECRET is required when the overlay relay is enabled")
		}
		callback, callbackErr := validateAbsoluteURL("OVERLAY_EVENTSUB_CALLBACK_URL", c.OverlayEventSubCallbackURL)
		if callbackErr != nil {
			problems = append(problems, callbackErr.Error())
		} else {
			if callback.Scheme != "https" && !isLoopbackHost(callback.Hostname()) {
				problems = append(problems, "OVERLAY_EVENTSUB_CALLBACK_URL must use HTTPS outside local development")
			}
			if callback.Path != "/api/overlay/eventsub" {
				problems = append(problems, "OVERLAY_EVENTSUB_CALLBACK_URL path must be /api/overlay/eventsub")
			}
		}
	}

	// Twitch Extension ingest: empty disables it, but a half-configured extension
	// (only one of client id / secret) is a mistake, and the secret must decode.
	if c.OverlayExtensionClientID != "" || c.OverlayExtensionSecret != "" {
		if strings.TrimSpace(c.OverlayExtensionClientID) == "" {
			problems = append(problems, "OVERLAY_EXTENSION_CLIENT_ID is required when the Twitch extension is enabled")
		}
		if strings.TrimSpace(c.OverlayExtensionSecret) == "" {
			problems = append(problems, "OVERLAY_EXTENSION_SECRET is required when the Twitch extension is enabled")
		} else if _, err := twitchext.DecodeSecret(c.OverlayExtensionSecret); err != nil {
			// Same lenient decode the runtime uses, so a valid (unpadded) Twitch
			// secret can never pass validation yet fail at use, or vice versa.
			problems = append(problems, "OVERLAY_EXTENSION_SECRET must be base64 (as shown in the Twitch developer console)")
		}
		if c.OverlayExtensionOrigin != "" {
			if _, err := validateAbsoluteURL("OVERLAY_EXTENSION_ORIGIN", c.OverlayExtensionOrigin); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid core-api configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

// ExtensionEnabled reports whether the Twitch Extension Bits ingest is
// configured. Mirrors the "empty disables it" gate used for the eventsub relay.
func (c CoreAPIConfig) ExtensionEnabled() bool {
	return strings.TrimSpace(c.OverlayExtensionClientID) != "" && strings.TrimSpace(c.OverlayExtensionSecret) != ""
}

// ExtensionAllowedOrigin is the browser origin the extension panel posts from,
// which CORS must allow. Hosted and Released extensions run at
// https://<client id>.ext-twitch.tv; OverlayExtensionOrigin overrides that for
// local testing. Empty when the extension is not configured.
func (c CoreAPIConfig) ExtensionAllowedOrigin() string {
	if !c.ExtensionEnabled() {
		return ""
	}
	if o := strings.TrimSpace(c.OverlayExtensionOrigin); o != "" {
		return strings.TrimRight(o, "/")
	}
	return "https://" + strings.TrimSpace(c.OverlayExtensionClientID) + ".ext-twitch.tv"
}

// OverlayDownloadLogins returns the set of lowercased Twitch logins allowed to
// download the overlay build, parsed from a comma-separated allowlist. Blank
// entries are ignored so a trailing comma or spacing is harmless.
func (c CoreAPIConfig) OverlayDownloadLogins() map[string]bool {
	set := map[string]bool{}
	for raw := range strings.SplitSeq(c.OverlayDownloadAllowlist, ",") {
		if login := strings.ToLower(strings.TrimSpace(raw)); login != "" {
			set[login] = true
		}
	}
	return set
}

// OverlayDownloadEnabled reports whether the gated overlay download is
// configured: a build path plus at least one allowed login. Mirrors the "empty
// disables it" gate used for the eventsub relay and the extension ingest.
func (c CoreAPIConfig) OverlayDownloadEnabled() bool {
	return strings.TrimSpace(c.OverlayDownloadPath) != "" && len(c.OverlayDownloadLogins()) > 0
}

func (c CoreAPIConfig) FrontendOrigin() string {
	u, err := url.Parse(c.FrontendBaseURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// The following methods implement the small configuration interface consumed
// by the internal JWT package without introducing an import cycle.
func (c CoreAPIConfig) AuthCookieSecure() bool   { return c.SessionCookieSecure }
func (c CoreAPIConfig) AuthCookieDomain() string { return c.SessionCookieDomain }
func (c CoreAPIConfig) AuthSessionTTL() time.Duration {
	return time.Duration(c.SessionTTLMinutes) * time.Minute
}

func validateAbsoluteURL(name, raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Hostname() == "" {
		return nil, fmt.Errorf("%s must be an absolute URL", name)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("%s must use http or https", name)
	}
	return u, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
