package routes

const (
	HelloWorld = "/"

	PostCreateUser = "/user"

	PostSignup = "/signup"
	PostLogin  = "/login"

	//
	// secure routes
	//

	PostLogout  = "/logout"
	GetUserData = "/profile"

	//
	// deprecated:
	//

	GetUser        = "/user/:id"
	PostUpdateUser = "/user/:id"
	DeleteUser     = "/user/:id"

	PostWeather = "/weather"
)
