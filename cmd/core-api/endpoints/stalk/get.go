package stalk

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	stalkSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/stalk"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewGetStalkEndpoint returns the logged-in channel's !stalk target.
func NewGetStalkEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
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
		rec, found, errGet := svc.Get(channel)
		if errGet != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error fetching stalk target for channel [%s]: %s", channel, errGet.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"found":        found,
			"target":       rec.TargetLogin,
			"last_message": rec.LastMessage,
			"set_by":       rec.SetBy,
		})
	}
}
