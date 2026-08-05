package bot

import (
	"errors"
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

func NewPutNewUser(nivekSvc nivek.NivekService) echo.HandlerFunc {
	userService := user.NewService(nivekSvc)
	return func(c echo.Context) error {
		var req user.User

		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "unrecongnized request body"})
		}

		user, err := userService.GetUserByBroadcasterId(*req.TwitchID)
		if user != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "user already exists"})
		}

		if err != nil && !errors.Is(err, db.ErrNoMoreRows) {
			return c.JSON(http.StatusInternalServerError, nil)
		}

		// we've confirmed the user doesn't exist - now create
		userService.CreateNewUser(user)
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
