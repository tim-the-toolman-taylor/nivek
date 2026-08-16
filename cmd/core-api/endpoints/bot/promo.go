package bot

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/promo"
)

// NewPostPromoCreateEndpoint saves a recurring message for a channel (bot side;
// the !newpromo chat permission check lives in the bot handler). The channel is
// a lowercased Twitch login.
func NewPostPromoCreateEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := promo.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel         string `json:"channel"`
			Message         string `json:"message"`
			IntervalSeconds int    `json:"interval_seconds"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" || req.Message == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel and message required"})
		}
		if req.IntervalSeconds <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "interval_seconds required"})
		}
		created, err := svc.Create(req.Channel, req.Message, req.IntervalSeconds)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, created)
	}
}

// NewPostPromoEditLastEndpoint replaces the message + interval of a channel's
// most recently touched promo (bot side; the !newpromo edit-last permission
// check lives in the bot handler). Responds {"found": false} when the channel
// has no promos.
func NewPostPromoEditLastEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := promo.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel         string `json:"channel"`
			Message         string `json:"message"`
			IntervalSeconds int    `json:"interval_seconds"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" || req.Message == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel and message required"})
		}
		if req.IntervalSeconds <= 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "interval_seconds required"})
		}
		found, err := svc.UpdateLast(req.Channel, req.Message, req.IntervalSeconds)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"found": found})
	}
}

// NewPostPromoDeleteLastEndpoint deletes a channel's most recently touched promo.
// Responds {"found": false} when the channel has no promos.
func NewPostPromoDeleteLastEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := promo.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel string `json:"channel"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel required"})
		}
		found, err := svc.RemoveLast(req.Channel)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"found": found})
	}
}

// NewGetPromosForBotEndpoint returns all enabled promos across every channel for
// the bot's scheduler to poll.
func NewGetPromosForBotEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := promo.NewService(nivekSvc)
	return func(c echo.Context) error {
		promos, err := svc.ListActive()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"promos": promos})
	}
}
