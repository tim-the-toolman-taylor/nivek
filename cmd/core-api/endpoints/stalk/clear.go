package stalk

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	stalkSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/stalk"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewClearStalkEndpoint removes the logged-in channel's !stalk target.
func NewClearStalkEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c, nivek.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		channel, errCh := channelLogin(user)
		if errCh != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errCh.Error()})
		}

		svc := stalkSvc.NewService(nivek)
		found, errClear := svc.Clear(channel)
		if errClear != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error clearing stalk target for channel [%s]: %s", channel, errClear.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{"found": found})
	}
}
