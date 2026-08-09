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
