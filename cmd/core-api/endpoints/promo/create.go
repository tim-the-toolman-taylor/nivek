package promo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	promoSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewCreatePromoEndpoint adds a recurring message scoped to the logged-in
// channel. interval_seconds is clamped to the service's bounds.
func NewCreatePromoEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
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
			Message         string `json:"message"`
			IntervalSeconds int    `json:"interval_seconds"`
		}
		if errBind := c.Bind(&payload); errBind != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
		payload.Message = strings.TrimSpace(payload.Message)
		if payload.Message == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "message required"})
		}
		if payload.IntervalSeconds <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "interval_seconds required"})
		}

		if user.TwitchID == nil || *user.TwitchID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "account has no linked twitch id"})
		}

		svc := promoSvc.NewService(nivek)
		created, errAdd := svc.Create(channel, *user.TwitchID, payload.Message, payload.IntervalSeconds)
		if errAdd != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error creating promo for channel [%s]: %s", channel, errAdd.Error()),
			})
		}

		return c.JSON(http.StatusOK, created)
	}
}
