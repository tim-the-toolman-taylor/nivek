package bot

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

func NewGetCommandsForBotEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	commandService := commands.NewService(nivekSvc)
	return func(c echo.Context) error {
		commands, err := commandService.GetGlobalEnabledCommands()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to fetch commands: %s", err.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{"commands": commands})
	}
}
