package promo

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	promoSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewGetPromosEndpoint returns the logged-in channel's recurring messages
// (enabled and disabled).
func NewGetPromosEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
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

		svc := promoSvc.NewService(nivek)
		promos, errList := svc.ListForChannel(channel)
		if errList != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error fetching promos for channel [%s]: %s", channel, errList.Error()),
			})
		}

		return c.JSON(http.StatusOK, promos)
	}
}
