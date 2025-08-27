package user

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

func NewUpdateUserEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var payload User
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("payload binding failed: %s", err.Error()),
			})
		}

		id := c.Param("id")

		err := nivek.Postgres().GetDefaultConnection().Collection(TableUser).
			Find(db.Cond{"id": id}).
			Update(payload)

		if err != nil {
			return c.JSON(
				http.StatusInternalServerError,
				map[string]string{
					"error": fmt.Sprintf("user db update failed: %s", err.Error()),
				},
			)
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("user (%s) updated successfully", id),
		})
	}
}
