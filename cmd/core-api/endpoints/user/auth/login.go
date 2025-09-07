package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	userLib "github.com/suuuth/nivek/internal/libraries/user"
	"github.com/upper/db/v4"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func NewLoginEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var loginRequest LoginRequest
		if err := c.Bind(&loginRequest); err != nil {
			return utility.RejectBadRequest(c)
		}

		var user userLib.User

		err := nivek.Postgres().
			GetDefaultConnection().
			Collection(userLib.TableUser).
			Find(db.Cond{
				"email":    loginRequest.Email,
				"password": loginRequest.Password,
			}).
			One(&user)

		if err != nil {
			logrus.Errorf(
				"error during login attempt - %s: %s",
				loginRequest.Email,
				err.Error(),
			)

			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "login failed",
			})
		}

		return c.JSON(http.StatusOK, user)
	}
}
