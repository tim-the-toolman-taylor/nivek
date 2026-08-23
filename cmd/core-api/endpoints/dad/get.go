package dad

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	dadSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/dad"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewGetDadResponsesEndpoint returns the logged-in channel's !dad pool: the
// shared globals plus the channel's own responses (with usage counts).
func NewGetDadResponsesEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		svc := dadSvc.NewService(nivek)
		responses, errList := svc.ListForChannel(*user.TwitchLogin)
		if errList != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching dad responses for user [%s]: %s",
					*user.TwitchLogin, errList.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, responses)
	}
}
