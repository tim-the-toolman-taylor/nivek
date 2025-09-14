package task

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/task"
	"github.com/suuuth/nivek/internal/libraries/utilities"
)

func NewPostCreateUserTaskEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		var createTaskRequest task.CreateTaskRequest
		if err := c.Bind(&createTaskRequest); err != nil {
			logrus.Errorf("failed to bind request body during create-task request: %s", err.Error())
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to read request body",
			})
		}

		taskService := task.NewNivekTaskService(nivek)
		tasks, err := taskService.CreateTask(user, &createTaskRequest)
		if err != nil {
			logrus.Errorf("failed to get tasks: %s", err.Error())
		}

		return c.JSON(http.StatusOK, tasks)
	}
}
