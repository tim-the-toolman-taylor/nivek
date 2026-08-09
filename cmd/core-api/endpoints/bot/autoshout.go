package bot

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	autoShoutSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

func NewGetAutoShoutChatters(nivekSvc nivek.NivekService) echo.HandlerFunc {
	autoShoutService := autoShoutSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		bidStr := c.Param("bid")

		chatters, errChat := autoShoutService.GetAutoShoutChattersForBot(bidStr)
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
