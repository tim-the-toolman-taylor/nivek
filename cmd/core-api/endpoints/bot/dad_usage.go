package bot

import (
	"net/http"

	"github.com/labstack/echo/v4"
	dadUsageSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/dadusage"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
)

type dadUsageRequest struct {
	TwitchID int `json:"twitch_id"`
}

// NewPostDadUsage returns a chatter -> rolls-served map for the broadcaster's
// current stream. The bot calls this on boot to rehydrate its in-memory !dad
// rate-limit counters so a restart mid-stream doesn't hand everyone a fresh
// allotment.
func NewPostDadUsage(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := dadUsageSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req dadUsageRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.TwitchID == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "twitch_id required"})
		}
		usage, err := svc.GetCurrentStreamUsage(req.TwitchID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "fetch usage failed"})
		}
		return c.JSON(http.StatusOK, usage)
	}
}

type dadIncrementRequest struct {
	TwitchID    int    `json:"twitch_id"`
	Chattername string `json:"chattername"`
}

// NewPostDadIncrement records one !dad roll for a chatter, stamping the
// broadcaster's current stream_key. Called by the bot on each counted roll
// (allow/reject); over-limit rolls stay in-process and never reach here.
func NewPostDadIncrement(nivekSvc nivek.NivekService) echo.HandlerFunc {
	svc := dadUsageSvc.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req dadIncrementRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "decode body"})
		}
		if req.TwitchID == 0 || req.Chattername == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "twitch_id and chattername required"})
		}
		if err := svc.IncrementRoll(req.TwitchID, req.Chattername); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "increment failed"})
		}
		return c.NoContent(http.StatusNoContent)
	}
}
