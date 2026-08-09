package bot

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	autoShoutSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

func NewGetAutoShoutChatters(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userSvc := user.NewService(nivekSvc)
	autoShoutService := autoShoutSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		bidStr := c.Param("bid")
		usr, err := userSvc.GetUserByBroadcasterId(bidStr)
		if err != nil {
			return c.JSON(
				http.StatusInternalServerError,
				map[string]string{
					"error": fmt.Sprintf(
						"failed to fetch user by bid %s - %s",
						c.Param("bid"),
						err.Error(),
					),
				},
			)
		}

		usrnm := usr.Username
		if usr.TwitchLogin != nil {
			usrnm = *usr.TwitchLogin
		}

		chatters, errChat := autoShoutService.GetAutoShoutChatters(usrnm)
		if errChat != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching auto shout chatter for user [%s]: %s",
					usrnm,
					errChat.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, chatters)
	}
}
