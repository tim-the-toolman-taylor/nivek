package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

func NewGetActiveChannelsEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		users, err := userService.GetAllActiveUsers()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "fetch active users",
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"channels": users})
	}
}

func NewPostHealLegacyUserEndpoint(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req user.User

		if err := c.Bind(&req); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}

		if req.TwitchLogin == nil || req.TwitchDisplayName == nil || req.TwitchID == nil {
			return c.NoContent(http.StatusBadRequest)
		}

		if err := userService.UpdateUser(&req); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to update user: %+v - %s", req, err.Error()),
			})
		}

		fresh, err := userService.GetUserByBroadcasterId(*req.TwitchID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{})
		}

		freshByte, err := json.Marshal(fresh)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to marshal fresh user record: %+v - %s", fresh, err.Error()),
			})
		}

		return c.JSON(http.StatusOK, freshByte)
	}
}

func NewPutNewUser(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req user.User

		if err := c.Bind(&req); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}

		user, err := userService.GetUserByBroadcasterId(*req.TwitchID)
		if user != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "user already exists"})
		}

		if err != nil && !errors.Is(err, db.ErrNoMoreRows) {
			return c.JSON(http.StatusInternalServerError, nil)
		}

		// we've confirmed the user doesn't exist - now create
		if err := userService.CreateNewUser(user); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("failed to write new user %+v: %s", user, err.Error()),
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}

func NewPutChannelState(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req struct {
			BroadcasterUserLogin *string `json:"twitch_login"`
			IsLive               bool    `json:"is_live"`
		}

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if req.BroadcasterUserLogin == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecognized request body"})
		}

		if err := userService.PutChannelState(*req.BroadcasterUserLogin, req.IsLive); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "something went wrong :(",
			})
		}
		return c.JSON(http.StatusNoContent, nil)
	}
}
