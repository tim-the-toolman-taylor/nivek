package stalk

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	stalkSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/stalk"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewSetStalkEndpoint upserts the logged-in channel's !stalk target.
func NewSetStalkEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
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

		var payload struct {
			Target string `json:"target"`
		}
		if errBind := c.Bind(&payload); errBind != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}

		target, ok := stalkSvc.NormalizeTarget(payload.Target)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "that doesn't look like a username"})
		}

		svc := stalkSvc.NewService(nivek)
		if errSet := svc.Set(channel, target, setByLogin(user)); errSet != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error setting stalk target for channel [%s]: %s", channel, errSet.Error()),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"target": target})
	}
}
