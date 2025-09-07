package user

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	user2 "github.com/suuuth/nivek/internal/libraries/user"
)

type CreateUserRequest struct {
	Username string `db:"username" json:"username"`
	Email    string `db:"email" json:"email"`
}

func NewCreateUserEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var user CreateUserRequest
		if err := c.Bind(&user); err != nil {
			return utility.RejectBadRequest(c)
		}

		result, err := nivek.Postgres().GetDefaultConnection().
			Collection(user2.TableUser).
			Insert(user)

		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("successfully created record with id: %d", result.ID()),
		})
	}
}
