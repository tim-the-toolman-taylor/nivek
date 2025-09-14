package utilities

import (
	"fmt"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	userlib "github.com/suuuth/nivek/internal/libraries/user"
)

func GetUserFromContext(c echo.Context) (*userlib.User, error) {
	user, ok := c.Get("user").(*userlib.User)
	if !ok {
		logrus.WithField(
			"stack",
			string(debug.Stack()),
		).Errorf("failed to get user from context")
		return nil, fmt.Errorf("failed to get user from context")
	}
	return user, nil
}
