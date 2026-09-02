package user

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// profileResponse is the signed-in user's profile plus a computed overlay_access
// flag: whether their Twitch login is on the overlay allowlist
// (OVERLAY_DOWNLOAD_ALLOWLIST). The frontend gates the /overlay page and its nav
// link on it, so the allowlist stays in one place (the backend env) instead of
// being duplicated in the client. The embedded *User flattens into the JSON, so
// existing profile consumers are unaffected.
type profileResponse struct {
	*userlib.User
	OverlayAccess bool `json:"overlay_access"`
}

// NewGetProfileEndpoint returns the user already resolved by JWT middleware.
// It deliberately does not parse the token a second time.
func NewGetProfileEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		userData, ok := c.Get("user").(*userlib.User)
		if !ok || userData == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authenticated user")
		}
		c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
		return c.JSON(http.StatusOK, profileResponse{
			User:          userData,
			OverlayAccess: overlayAccess(svc, userData),
		})
	}
}

// overlayAccess reports whether the user's Twitch login is on the overlay
// allowlist. It reuses the download allowlist as the single set of overlay
// testers, so page access and build download can never drift apart.
func overlayAccess(svc nivek.NivekService, u *userlib.User) bool {
	if u.TwitchLogin == nil {
		return false
	}
	cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
	if !ok {
		return false
	}
	login := strings.ToLower(strings.TrimSpace(*u.TwitchLogin))
	return login != "" && cfg.OverlayDownloadLogins()[login]
}
