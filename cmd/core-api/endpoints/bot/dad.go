package bot

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/dad"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

// NewPostDadRandomEndpoint returns a random !dad response for the channel and
// increments its use_count. Empty response (no rows) is a valid 200 — the bot
// simply stays quiet.
func NewPostDadRandomEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := dad.NewService(nivekSvc)
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
		response, err := svc.PickRandom(req.Channel)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"response": response})
	}
}

// NewPostDadAddEndpoint adds a channel-scoped !dad response (bot side; chat
// permission checks live in the bot handler).
func NewPostDadAddEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := dad.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel  string `json:"channel"`
			Response string `json:"response"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" || req.Response == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel and response required"})
		}
		created, err := svc.Add(req.Channel, req.Response)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, created)
	}
}

// NewPostDadRemoveEndpoint removes one of the channel's own !dad responses.
func NewPostDadRemoveEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := dad.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel string `json:"channel"`
			ID      int    `json:"id"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" || req.ID == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel and id required"})
		}
		if err := svc.Remove(req.Channel, req.ID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{})
	}
}
