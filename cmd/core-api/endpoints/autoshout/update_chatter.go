package autoshout

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	autoShoutSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

func NewUpdateAutoShoutChatterEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		var updatedChatter autoShoutSvc.ShoutChatter
		if errBind := c.Bind(&updatedChatter); errBind != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
		}

		bid, errBid := broadcasterID(user)
		if errBid != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": errBid.Error()})
		}

		autoShoutService := autoShoutSvc.NewService(nivek)

		// get chatter record to make sure they exist first @TODO::update this to use a smaller fetch
		chatter, errFetch := autoShoutService.GetAutoShoutChatter(bid, updatedChatter.ChatterName)
		if errFetch != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "internal server error",
			})
		}

		if errChat := autoShoutService.UpdateAutoShoutChatter(chatter); errChat != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error deleting auto shout chatter for user [%d]: %s",
					user.Id, errChat.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{})
	}
}
