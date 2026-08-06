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

	// Bot listener, reached over the Docker gateway.
	BotInternalURL string `envconfig:"BOT_INTERNAL_URL" default:"http://172.19.0.1:8090"`
	BotAPIHMACKey  string `envconfig:"BOT_API_HMAC_KEY" default:""`
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

	if len(problems) > 0 {
		return fmt.Errorf("invalid core-api configuration: %s", strings.Join(problems, "; "))
	}
	return nil
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
