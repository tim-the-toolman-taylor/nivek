package task

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/task"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

func NewGetUserTasksEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c, nivek.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		taskService := task.NewNivekTaskService(nivek)
		tasks, err := taskService.GetTasks(user)
		if err != nil {
			nivek.Logger().Errorf("failed to get tasks: %s", err.Error())
		}

		return c.JSON(http.StatusOK, tasks)
	}
}
