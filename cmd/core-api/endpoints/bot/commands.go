package bot

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	commandSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
)

// NewGetChannelCommands serves the enabled channel-scoped custom commands for a
// single broadcaster (by Twitch id in :bid). The bot calls this when a channel
// goes live — and at boot for channels already live — to load that channel's
// custom triggers. botAuth-gated: unlike the public /commands (global) fetch,
// this is bot-only plumbing, but the data isn't sensitive either way.
//
// The response also carries the channel's capability set. Capability-gated
// global commands (nivek.command.requires) dispatch only in channels that hold
// the named capability, and the bot needs that answer per channel. It rides
// this response rather than a second endpoint so it loads on exactly the
// lifecycle the bot already has -- fetched on stream.online, dropped on
// stream.offline -- with no extra round trip.
func NewGetChannelCommands(nivekSvc nivek.NivekService) echo.HandlerFunc {
	commandsService := commandSvc.NewService(nivekSvc)
	relay := overlayrelay.NewService(nivekSvc)
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

		// A capability lookup failing is not worth failing the whole load over:
		// the channel's custom commands still work, and the gated globals stay
		// off, which is the safe direction.
		capabilities := []string{}
		hasOverlay, err := relay.HasActiveDevice(bid)
		if err != nil {
			nivekSvc.Logger().Errorf("channel commands: overlay capability for %s: %s", bid, err.Error())
		} else if hasOverlay {
			capabilities = append(capabilities, commandSvc.CapabilityOverlay)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"commands":     cmds,
			"capabilities": capabilities,
		})
	}
}
