package df

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overseer/wire"
)

// NewPostSnapshotEndpoint accepts a MapSnapshot JSON body, verifies the
// HMAC in the X-Nivek-HMAC header against DASHBOARD_PUSH_HMAC_KEY, and
// replaces the in-memory store. The key is read per-request so the user
// can rotate it (edit env, restart core-api) without code changes.
//
// Failure modes return distinct status codes so the pusher's logs are
// useful for debugging:
//
//	503 — server's HMAC key isn't configured (deploy issue)
//	400 — body unreadable, HMAC header malformed, or JSON decode fails
//	401 — HMAC header missing or doesn't match
//	200 — accepted and stored
func NewPostSnapshotEndpoint(_ nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		keyHex := os.Getenv("DASHBOARD_PUSH_HMAC_KEY")
		if keyHex == "" {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "push endpoint not configured (server-side hmac key missing)",
			})
		}
		key, err := hex.DecodeString(keyHex)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "server hmac key not valid hex",
			})
		}

		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "read body: " + err.Error(),
			})
		}

		// Decompress if the pusher gzipped the body. HMAC is computed over
		// the UNCOMPRESSED JSON so we decompress before verifying.
		if c.Request().Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "gzip reader: " + err.Error(),
				})
			}
			body, err = io.ReadAll(gz)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "gunzip body: " + err.Error(),
				})
			}
			_ = gz.Close()
		}

		claimedHex := c.Request().Header.Get("X-Nivek-HMAC")
		if claimedHex == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "missing X-Nivek-HMAC header",
			})
		}
		claimed, err := hex.DecodeString(claimedHex)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "X-Nivek-HMAC is not valid hex",
			})
		}

		mac := hmac.New(sha256.New, key)
		mac.Write(body)
		if !hmac.Equal(claimed, mac.Sum(nil)) {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "hmac mismatch",
			})
		}

		var snap wire.MapSnapshot
		if err := json.Unmarshal(body, &snap); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "decode snapshot json: " + err.Error(),
			})
		}

		store.set(&snap)
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}
