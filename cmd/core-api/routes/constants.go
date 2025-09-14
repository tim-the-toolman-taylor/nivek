package routes

const (
	HelloWorld = "/"

	PostCreateUser = "/user"

	PostSignup = "/signup"
	PostLogin  = "/login"

	//
	// secure routes
	//

	PostLogout         = "/logout"
	PostFetchUserData  = "/profile"
	GetUserTasks       = "/user/:id/task"
	PostCreateUserTask = "/user/:id/task"

	PostWeather = "/weather"
)
