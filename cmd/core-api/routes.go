package main

import (
	"net/http"
	
	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

func RegisterRoutes(nivek nivek.NivekService, e *echo.Echo) {
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
}
