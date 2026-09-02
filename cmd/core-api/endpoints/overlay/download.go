package overlay

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/coreconfig"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// downloadFilename is the name the browser saves the build as, independent of
// whatever path OVERLAY_DOWNLOAD_PATH points at on disk.
const downloadFilename = "RidiculousStream-windows.zip"

// NewDownloadEndpoint serves the overlay build to a hand-picked allowlist of
// Twitch logins. It sits behind the session-JWT middleware, so the caller is
// already authenticated; this only layers the allowlist check on top. It is the
// redistribution-POC gate: downloads are tied to real accounts (the same ones
// that mint device tokens), revocable per login without a shared password, and
// logged so we can see who pulled the build.
//
// Disabled (404) unless both OVERLAY_DOWNLOAD_PATH and OVERLAY_DOWNLOAD_ALLOWLIST
// are set, so a deployment with no build configured simply has no route rather
// than advertising a gated file.
func NewDownloadEndpoint(svc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, ok := svc.CustomConfig().(coreconfig.CoreAPIConfig)
		if !ok || !cfg.OverlayDownloadEnabled() {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
		}

		account, err := utilities.GetUserFromContext(c, svc.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		// The allowlist keys on Twitch login. A signed-in account without one is
		// never on it (should not happen, but fail closed).
		login := ""
		if account.TwitchLogin != nil {
			login = strings.ToLower(strings.TrimSpace(*account.TwitchLogin))
		}
		if login == "" || !cfg.OverlayDownloadLogins()[login] {
			svc.Logger().Infof("overlay download denied: user=%d login=%q not on allowlist", account.Id, login)
			return c.JSON(http.StatusForbidden, map[string]string{"error": "not authorized to download the overlay build"})
		}

		// Confirm the file before claiming success, so a bad mount/path is a clear
		// 500 rather than a zero-byte download to an allowed user.
		path := cfg.OverlayDownloadPath
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			svc.Logger().Errorf("overlay download: build not readable at %q: %v", path, statErr)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "build not available"})
		}

		svc.Logger().Infof("overlay download: user=%d login=%q served %s (%d bytes)", account.Id, login, filepath.Base(path), info.Size())
		// Attachment sets Content-Disposition so the browser saves under a stable
		// name, and streams via ServeContent (range/resume) for the ~50MB file.
		return c.Attachment(path, downloadFilename)
	}
}
