package bot

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	autoShoutSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

func NewGetAutoShoutChatters(nivekSvc nivek.NivekService) echo.HandlerFunc {
	autoShoutService := autoShoutSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		bidStr := c.Param("bid")

		bid, err := strconv.Atoi(bidStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("failed to convert bid from string to int %s - %s", bidStr, err.Error()),
			})
		}

		chatters, errChat := autoShoutService.GetAutoShoutChattersForBot(bid)
		if errChat != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching auto shout chatter for user [%s]: %s",
					bidStr,
					errChat.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, chatters)
	}
}

type autoShoutIncrementRequest struct {
	TwitchID    int    `json:"twitch_id"`
	Chattername string `json:"chattername"`
}

// NewPostAutoShoutIncrement bumps a chatter's shout_count for a broadcaster.
// Called by the bot each time it fires an auto-shoutout.
func NewPostAutoShoutIncrement(nivekSvc nivek.NivekService) echo.HandlerFunc {
	autoShoutService := autoShoutSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req autoShoutIncrementRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.TwitchID == 0 || req.Chattername == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "twitch_id and chattername required"})
		}
		if err := autoShoutService.IncrementShoutCount(req.TwitchID, req.Chattername); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "increment failed"})
		}
		return c.NoContent(http.StatusNoContent)
	}
}
