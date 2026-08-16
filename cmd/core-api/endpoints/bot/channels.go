package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

func NewGetActiveChannelsEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		users, err := userService.GetAllActiveUsers()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to fetch active users with error: %s", err.Error()),
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"channels": users})
	}
}

func NewPostHealLegacyUserEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req user.User

		if err := c.Bind(&req); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}

		if req.TwitchLogin == nil || req.TwitchDisplayName == nil || req.TwitchID == nil {
			return c.NoContent(http.StatusBadRequest)
		}

		if err := userService.UpdateUser(&req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to update user: %+v - %s", req, err.Error()),
			})
		}

		fresh, err := userService.GetUserByBroadcasterId(*req.TwitchID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{})
		}

		freshByte, err := json.Marshal(fresh)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to marshal fresh user record: %+v - %s", fresh, err.Error()),
			})
		}

		return c.JSON(http.StatusOK, freshByte)
	}
}

func NewPutNewUser(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req user.User

		if err := c.Bind(&req); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}

		existing, err := userService.GetUserByBroadcasterId(*req.TwitchID)
		if err != nil && !errors.Is(err, db.ErrNoMoreRows) {
			return c.JSON(http.StatusInternalServerError, nil)
		}

		// If the user already exists (e.g. previously !banish'd, which leaves the
		// row in place with bot_opt_in=false), treat !joinme as an idempotent
		// un-banish: flip opt-in back on using the already-fetched row rather than
		// erroring or inserting a duplicate.
		if existing != nil {
			existing.BotOptIn = true
			if err := userService.UpdateUser(existing); err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{
					"error": fmt.Sprintf("failed to re-opt-in existing user %+v: %s", existing, err.Error()),
				})
			}
			return c.JSON(http.StatusNoContent, nil)
		}

		// User doesn't exist - create. Pass the bound request, not the nil user
		// returned by the not-found lookup above.
		if err := userService.CreateNewUser(&req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to write new user %+v: %s", req, err.Error()),
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}

// NewPostOptOut opts a channel out of the bot (bot_opt_in=false) so it is no
// longer returned by GetActiveChannels at boot. Backs the !banish command. The
// user row and its data are preserved.
func NewPostOptOut(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			TwitchLogin *string `json:"twitch_login"`
		}

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if req.TwitchLogin == nil || *req.TwitchLogin == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing twitch_login"})
		}

		if err := userService.SetBotOptIn(*req.TwitchLogin, false); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to opt out user",
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}

// NewPostOptInCheck reports whether a channel currently has bot_opt_in=true.
// Backs the go-live handler's decision to (re)join an untracked channel.
func NewPostOptInCheck(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			TwitchLogin *string `json:"twitch_login"`
		}

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if req.TwitchLogin == nil || *req.TwitchLogin == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing twitch_login"})
		}

		optedIn, err := userService.IsBotOptIn(*req.TwitchLogin)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to check opt-in",
			})
		}
		return c.JSON(http.StatusOK, map[string]bool{"opted_in": optedIn})
	}
}

func NewPutChannelState(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			BroadcasterUserLogin *string `json:"twitch_login"`
			IsLive               bool    `json:"is_live"`
		}

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if req.BroadcasterUserLogin == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if err := userService.PutChannelState(*req.BroadcasterUserLogin, req.IsLive); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "something went wrong :(",
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}
