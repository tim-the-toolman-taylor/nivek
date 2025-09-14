package task

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/jwt"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/task"
)

func NewGetUserTasksEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		tokenString := strings.TrimPrefix(
			c.Request().Header.Get("Authorization"),
			"Bearer ",
		)

		jwtService := jwt.NewJWTService(nivek)
		user, err := jwtService.GetUserData(tokenString)
		if err != nil {
			logrus.Errorf("failed to get user data from token string: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to get user data",
			})
		}

		taskService := task.NewNivekTaskService(nivek)
		tasks, err := taskService.GetTasks(user)
		if err != nil {
			logrus.Errorf("failed to get tasks: %s", err.Error())
		}

		return c.JSON(http.StatusOK, tasks)
	}
}
