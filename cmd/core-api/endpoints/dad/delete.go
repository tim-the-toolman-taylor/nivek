package dad

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	dadSvc "github.com/tim-the-toolman-taylor/nivek/internal/libraries/dad"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

// NewDeleteDadResponseEndpoint removes one of the logged-in channel's own !dad
// responses. Globals and other channels' rows are unaffected (enforced in the
// service).
func NewDeleteDadResponseEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		id, errId := strconv.Atoi(c.Param("id"))
		if errId != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid id parameter",
			})
		}

		svc := dadSvc.NewService(nivek)
		if errDel := svc.Remove(*user.TwitchLogin, id); errDel != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error deleting dad response for user [%s]: %s",
					*user.TwitchLogin, errDel.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{})
	}
}
