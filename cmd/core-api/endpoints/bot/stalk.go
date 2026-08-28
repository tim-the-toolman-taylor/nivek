package bot

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/stalk"
)

// NewGetStalkTargetEndpoint returns the channel's configured !stalk target.
// Responds {"found": false} when the channel has no target yet.
func NewGetStalkTargetEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := stalk.NewService(nivekSvc)
	return func(c echo.Context) error {
		channel := c.QueryParam("channel")
		if channel == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel query param required"})
		}
		target, found, err := svc.Get(channel)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"found": found, "target": target})
	}
}

// NewPostStalkSetEndpoint upserts a channel's !stalk target (bot side; the
// mod/broadcaster check lives in the bot handler).
func NewPostStalkSetEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := stalk.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			Channel string `json:"channel"`
			Target  string `json:"target"`
			SetBy   string `json:"set_by"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.Channel == "" || req.Target == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "channel and target required"})
		}
		if err := svc.Set(req.Channel, req.Target, req.SetBy); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"target": req.Target})
	}
}

// NewPostStalkClearEndpoint deletes a channel's !stalk target. Responds
// {"found": false} when the channel had no target.
func NewPostStalkClearEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := stalk.NewService(nivekSvc)
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
		found, err := svc.Clear(req.Channel)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"found": found})
	}
}
