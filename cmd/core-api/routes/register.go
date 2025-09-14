package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/cmd/core-api/endpoints"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/task"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/user"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/user/auth"
	"github.com/suuuth/nivek/cmd/core-api/endpoints/weather"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/nivekmiddleware"
)

func RegisterRoutes(nivek nivek.NivekService, e *echo.Echo) {

	//
	// Hello World
	e.GET(HelloWorld, endpoints.NewIndexEndpoint(nivek))

	e.POST(PostCreateUser, user.NewCreateUserEndpoint(nivek))

	//
	// Login, Signup
	e.POST(PostSignup, auth.NewSignupEndpoint(nivek))
	e.POST(PostLogin, auth.NewLoginEndpoint(nivek))

	//
	// Secure routes:
	e.POST(PostLogout, auth.NewLogoutEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.POST(PostFetchUserData, user.NewGetProfileEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.GET(GetUserTasks, task.NewGetUserTasksEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.POST(PostCreateUserTask, task.NewPostCreateUserTaskEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// weather
	e.POST(PostWeather, weather.NewGetWeatherEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)
}
