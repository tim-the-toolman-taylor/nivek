package bot

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// NewGetChannelsEndpoint returns the list of active usernames the twitch-bot
// should join. Replaces the bot's direct `userService.GetAllActiveUsers()`
// call so the Pi no longer needs DB access. The bot still applies its own
// Twitch-IRC validity filter on the returned list (that's a chat-protocol
// concern, not a server one).
func NewGetChannelsEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		users, err := userService.GetAllActiveUsers()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "fetch active users",
			})
		}
		channels := make([]string, 0, len(users))
		for _, u := range users {
			channels = append(channels, u.Username)
		}
		return c.JSON(http.StatusOK, map[string]any{"channels": channels})
	}
}

func NewGetActiveChannelsEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		users, err := userService.GetAllActiveUsers()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "fetch active users",
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"channels": users})
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
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}

		if req.BroadcasterUserLogin == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "twitch_login is required"})
		}

		if err := userService.PutChannelState(*req.BroadcasterUserLogin, req.IsLive); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "something went wrong :(",
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}
