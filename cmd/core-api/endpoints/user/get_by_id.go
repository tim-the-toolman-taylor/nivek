package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/upper/db/v4"
)

func NewGetUserByIdEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")

		var user User

		err := nivek.Postgres().
			GetDefaultConnection().
			Collection(TableUser).
			Find(db.Cond{"id": id}).
			One(&user)

		if err != nil {
			log.Errorf("failed fetching user where ID = %s: %s", id, err.Error())
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "user not found",
			})
		}

		return c.JSON(http.StatusOK, user)
	}
}
