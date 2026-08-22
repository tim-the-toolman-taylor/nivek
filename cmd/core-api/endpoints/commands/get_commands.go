package commands

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	commandSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/commands"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

func NewGetCommandsEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		commandsService := commandSvc.NewService(nivek)
		cmds, err := commandsService.GetGlobalEnabledCommands()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to fetch commands: %s", err.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{"commands": cmds})
	}
}
