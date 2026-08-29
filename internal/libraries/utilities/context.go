package utilities

import (
	"fmt"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// GetUserFromContext returns the authenticated user the JWT middleware stored on
// the request context.
//
// The caller passes the service logger (nivek.Logger()) instead of this package
// reaching for the package-level logrus one. Only the service logger carries the
// Discord error hook core-api registers at startup, so a package-level
// logrus.Errorf here would write to a different logger and never page anyone.
func GetUserFromContext(c echo.Context, logger *logrus.Logger) (*userlib.User, error) {
	user, ok := c.Get("user").(*userlib.User)
	if !ok {
		logger.WithField(
			"stack",
			string(debug.Stack()),
		).Errorf("failed to get user from context")
		return nil, fmt.Errorf("failed to get user from context")
	}
	return user, nil
}
