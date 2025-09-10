package routes

const (
	HelloWorld = "/"

	PostCreateUser = "/user"

	PostSignup = "/signup"
	PostLogin  = "/login"

	//
	// secure routes
	//

	PostLogout = "/logout"

	GetUser        = "/user/:id"
	PostUpdateUser = "/user/:id"
	DeleteUser     = "/user/:id"

	PostWeather = "/weather"
)
