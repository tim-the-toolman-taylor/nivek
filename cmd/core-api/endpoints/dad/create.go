package dad

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	dadSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/dad"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewCreateDadResponseEndpoint adds a !dad response scoped to the logged-in
// channel.
func NewCreateDadResponseEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c, nivek.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		var payload struct {
			Response string `json:"response"`
		}
		if errBind := c.Bind(&payload); errBind != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
		}
		if payload.Response == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "response required",
			})
		}

		if user.TwitchLogin == nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("missing twitch_login for user %d", user.Id),
			})
		}

		svc := dadSvc.NewService(nivek)
		if _, errAdd := svc.Add(*user.TwitchLogin, payload.Response); errAdd != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error adding dad response for user [%s]: %s",
					*user.TwitchLogin, errAdd.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{})
	}
}
