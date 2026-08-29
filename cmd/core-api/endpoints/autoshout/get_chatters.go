package autoshout

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	autoShoutSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

func NewGetAutoShoutChattersEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {

		user, err := utilities.GetUserFromContext(c, nivekSvc.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		bid, errBid := broadcasterID(user)
		if errBid != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errBid.Error()})
		}

		autoShoutService := autoShoutSvc.NewService(nivekSvc)
		chatters, errChat := autoShoutService.GetAutoShoutChatters(bid)
		if errChat != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching auto shout chatter for user [%d]: %s",
					user.Id, errChat.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, chatters)
	}
}
