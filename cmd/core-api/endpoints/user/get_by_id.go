package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userLib "github.com/suuuth/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

//
// @DEPRECATED
// needs rewrite with user service
//

func NewGetUserByIdEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := c.Param("id")

		var user userLib.User

		err := nivek.Postgres().
			GetDefaultConnection().
			Collection(userLib.TableUser).
			Find(db.Cond{"id": id}).
			One(&user)

		if err != nil {
			logrus.Errorf("failed fetching user where ID = %s: %s", id, err.Error())
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "user not found",
			})
		}

		return c.JSON(http.StatusOK, user)
	}
}
