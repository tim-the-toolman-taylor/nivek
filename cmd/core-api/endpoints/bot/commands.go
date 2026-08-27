package bot

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	commandSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

// NewGetChannelCommands serves the enabled channel-scoped custom commands for a
// single broadcaster (by Twitch id in :bid). The bot calls this when a channel
// goes live — and at boot for channels already live — to load that channel's
// custom triggers. botAuth-gated: unlike the public /commands (global) fetch,
// this is bot-only plumbing, but the data isn't sensitive either way.
func NewGetChannelCommands(nivekSvc nivek.NivekService) echo.HandlerFunc {
	commandsService := commandSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		bid := c.Param("bid")
		if bid == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "missing broadcaster twitch id",
			})
		}

		cmds, err := commandsService.GetChannelEnabledCommands(bid)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to fetch channel commands for %s: %s", bid, err.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{"commands": cmds})
	}
}
