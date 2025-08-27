package user

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

func NewDeleteUserEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")

		err := nivek.Postgres().GetDefaultConnection().
			Collection(TableUser).
			Find(db.Cond{"id": id}).
			Delete()

		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("successfully deleted record with id: %s", id),
		})
	}
}
