package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/bot"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/df"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/fishing"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/messenger"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/task"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user/auth"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/weather"
	apilib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivekmiddleware"
)

// RegisterRoutes attaches the API handlers to the given router group. It takes
// an *echo.Group (rather than *echo.Echo) so the API can be mounted under a
// path prefix — e.g. e.Group("/api") — leaving the root free for the static
// SPA served by middleware.Static (Traefik routes both to the same container).
func RegisterRoutes(nivek nivek.NivekService, e *echo.Group) {

	//
	// Hello World
	e.GET(apilib.HelloWorld, endpoints.NewIndexEndpoint(nivek))

	//
	// Auth — Twitch OAuth is the only signup/login path. /start redirects to
	// Twitch with a CSRF state cookie; /callback exchanges the code, fetches
	// the user's Twitch profile, find-or-creates a row, issues our JWT, and
	// 302s back to the frontend with the token in the URL fragment.
	e.GET(apilib.GetTwitchStart, auth.NewTwitchStartEndpoint(nivek))
	e.GET(apilib.GetTwitchCallback, auth.NewTwitchCallbackEndpoint(nivek))

	//
	// Secure routes:
	e.POST(apilib.PostLogout, auth.NewLogoutEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.POST(apilib.PostFetchUserData, user.NewGetProfileEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.GET(apilib.GetUserTasks, task.NewGetUserTasksEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	e.POST(apilib.PostCreateUserTask, task.NewPostCreateUserTaskEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// weather
	e.POST(apilib.PostWeather, weather.NewGetWeatherEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// fishing
	e.GET(apilib.GetFishingScore, fishing.NewGetFishingScoreEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// auto shout
	e.GET(apilib.GetAutoShoutChatters, autoshout.NewGetAutoShoutChattersEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)
	e.POST(apilib.PostCreateAutoShoutChatter, autoshout.NewCreateAutoShoutChatterEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)
	e.POST(apilib.PostUpdateAutoShoutChatter, autoshout.NewUpdateAutoShoutChatterEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)
	e.DELETE(apilib.DeleteAutoShoutChatter, autoshout.NewDeleteAutoShoutChatterEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// messager
	e.POST(apilib.PostCreateMessage, messenger.NewCreateMesageEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)
	e.GET(apilib.GetMessages, messenger.NewGetMessagesEndpoint(nivek),
		nivekmiddleware.NewJWTMiddleware(nivek).Middleware(),
	)

	// DF dashboard (GET public, POST HMAC-authed in the handler)
	e.GET(apilib.GetDFSnapshot, df.NewGetSnapshotEndpoint(nivek))
	e.POST(apilib.PostDFSnapshot, df.NewPostSnapshotEndpoint(nivek))

	// twitch-bot RPC. HMAC-authed via BOT_API_HMAC_KEY (hex). See
	// nivekmiddleware.NewHMACMiddleware for the canonical-string format the
	// bot signs.
	botAuth := nivekmiddleware.NewHMACMiddleware("BOT_API_HMAC_KEY")
	e.GET(apilib.GetBotChannels, bot.NewGetChannelsEndpoint(nivek), botAuth)
	e.GET(apilib.GetActiveChannels, bot.NewGetActiveChannelsEndpoint(nivek), botAuth)
	e.POST(apilib.PostBotBreadIncrement, bot.NewPostBreadIncrementEndpoint(nivek), botAuth)
	e.GET(apilib.GetBotBreadTotal, bot.NewGetBreadTotalEndpoint(nivek), botAuth)
	e.POST(apilib.PostBotLurkMessage, bot.NewPostLurkMessageEndpoint(nivek), botAuth)
	e.POST(apilib.PostBotFishGo, bot.NewPostFishGoEndpoint(nivek), botAuth)
	e.PUT(apilib.PutBroadcasterState, bot.NewPutChannelState(nivek), botAuth)
}
