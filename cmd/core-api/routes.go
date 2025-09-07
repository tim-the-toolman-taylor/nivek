package main

import (
	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/cmd/core-api/endpoints"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/user"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/user/auth"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/weather"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

func RegisterRoutes(nivek nivek.NivekService, e *echo.Echo) {
	e.GET("/", endpoints.NewIndexEndpoint(nivek))

	//
	// Basic CRUD user methods
	e.GET("/user/:id", user.NewGetUserByIdEndpoint(nivek))
	e.POST("/user", user.NewCreateUserEndpoint(nivek))
	e.POST("/user/:id", user.NewUpdateUserEndpoint(nivek))
	e.DELETE("/user/:id", user.NewDeleteUserEndpoint(nivek))

	//
	// Auth
	e.POST("/login", auth.NewLoginEndpoint(nivek))
	e.POST("/logout", auth.NewLogoutEndpoint(nivek))

	//
	// Weather
	e.POST("/weather", weather.NewGetWeatherEndpoint(nivek))
}
