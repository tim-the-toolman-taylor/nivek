package endpoints

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type User struct {
	id        int
	username  string
	email     string
	createdAt string
}

func NewIndexEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello world")
	}
}
