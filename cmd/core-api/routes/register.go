package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/autoshout"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/bot"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/commands"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/dad"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/df"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/fishing"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/messenger"
	promoEp "github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/promo"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/task"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user/auth"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/weather"
	apilib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivekmiddleware"
)

func RegisterRoutes(svc nivek.NivekService, e *echo.Group) {
	e.GET(apilib.HelloWorld, endpoints.NewIndexEndpoint(svc))

	// OAuth endpoints are public. They create the local HttpOnly session cookie
	// only after state validation and a successful Twitch identity lookup.
	e.GET(apilib.GetTwitchStart, auth.NewTwitchStartEndpoint(svc))
	e.GET(apilib.GetTwitchCallback, auth.NewTwitchCallbackEndpoint(svc))

	// Logout must remain callable with an expired or corrupt JWT so the browser
	// can always clear stale cookies.
	e.POST(apilib.PostLogout, auth.NewLogoutEndpoint(svc))

	e.GET(apilib.GetPublicCommands, commands.NewGetCommandsEndpoint(svc))

	authenticated := nivekmiddleware.NewJWTMiddleware(svc).Middleware()
	e.GET(apilib.GetUserProfile, user.NewGetProfileEndpoint(svc), authenticated)
	e.GET(apilib.GetUserTasks, task.NewGetUserTasksEndpoint(svc), authenticated)
	e.POST(apilib.PostCreateUserTask, task.NewPostCreateUserTaskEndpoint(svc), authenticated)
	e.POST(apilib.PostWeather, weather.NewGetWeatherEndpoint(svc), authenticated)
	e.GET(apilib.GetFishingScore, fishing.NewGetFishingScoreEndpoint(svc), authenticated)

	e.GET(apilib.GetAutoShoutChatters, autoshout.NewGetAutoShoutChattersEndpoint(svc), authenticated)
	e.POST(apilib.PostCreateAutoShoutChatter, autoshout.NewCreateAutoShoutChatterEndpoint(svc), authenticated)
	e.POST(apilib.PostUpdateAutoShoutChatter, autoshout.NewUpdateAutoShoutChatterEndpoint(svc), authenticated)
	e.DELETE(apilib.DeleteAutoShoutChatter, autoshout.NewDeleteAutoShoutChatterEndpoint(svc), authenticated)

	e.POST(apilib.PostCreateMessage, messenger.NewCreateMesageEndpoint(svc), authenticated)
	e.GET(apilib.GetMessages, messenger.NewGetMessagesEndpoint(svc), authenticated)

	e.GET(apilib.GetDadResponses, dad.NewGetDadResponsesEndpoint(svc), authenticated)
	e.POST(apilib.PostCreateDadResponse, dad.NewCreateDadResponseEndpoint(svc), authenticated)
	e.DELETE(apilib.DeleteDadResponse, dad.NewDeleteDadResponseEndpoint(svc), authenticated)

	e.GET(apilib.GetPromos, promoEp.NewGetPromosEndpoint(svc), authenticated)
	e.POST(apilib.PostCreatePromo, promoEp.NewCreatePromoEndpoint(svc), authenticated)
	e.POST(apilib.PostUpdatePromo, promoEp.NewUpdatePromoEndpoint(svc), authenticated)
	e.DELETE(apilib.DeletePromo, promoEp.NewDeletePromoEndpoint(svc), authenticated)

	// Public DF dashboard and HMAC-authenticated ingest.
	e.GET(apilib.GetDFSnapshot, df.NewGetSnapshotEndpoint(svc))
	e.POST(apilib.PostDFSnapshot, df.NewPostSnapshotEndpoint(svc))

	botAuth := nivekmiddleware.NewHMACMiddleware("BOT_API_HMAC_KEY")
	e.GET(apilib.GetActiveChannels, bot.NewGetActiveChannelsEndpoint(svc), botAuth)
	e.POST(apilib.PostHealLegacyUser, bot.NewPostHealLegacyUserEndpoint(svc), botAuth)
	e.GET(apilib.GetCommands, bot.NewGetCommandsForBotEndpoint(svc), botAuth)
	e.POST(apilib.PostBotBreadIncrement, bot.NewPostBreadIncrementEndpoint(svc), botAuth)
	e.GET(apilib.GetBotBreadTotal, bot.NewGetBreadTotalEndpoint(svc), botAuth)
	e.POST(apilib.PostBotLurkMessage, bot.NewPostLurkMessageEndpoint(svc), botAuth)
	e.POST(apilib.PostBotFishGo, bot.NewPostFishGoEndpoint(svc), botAuth)
	e.PUT(apilib.PutBroadcasterState, bot.NewPutChannelState(svc), botAuth)
	e.POST(apilib.PostBotOptOut, bot.NewPostOptOut(svc), botAuth)
	e.POST(apilib.PostBotOptInCheck, bot.NewPostOptInCheck(svc), botAuth)
	e.PUT(apilib.PutCreateNewUser, bot.NewPutNewUser(svc), botAuth)
	e.POST(apilib.PostBotDadRandom, bot.NewPostDadRandomEndpoint(svc), botAuth)
	e.POST(apilib.PostBotDadAdd, bot.NewPostDadAddEndpoint(svc), botAuth)
	e.POST(apilib.PostBotDadRemove, bot.NewPostDadRemoveEndpoint(svc), botAuth)
	e.POST(apilib.PostBotDadUsage, bot.NewPostDadUsage(svc), botAuth)
	e.POST(apilib.PostBotDadIncrement, bot.NewPostDadIncrement(svc), botAuth)
	e.GET(apilib.GetBotAutoShouters, bot.NewGetAutoShoutChatters(svc), botAuth)
	e.POST(apilib.PostBotAutoShoutIncrement, bot.NewPostAutoShoutIncrement(svc), botAuth)
	e.POST(apilib.PostBotPromoCreate, bot.NewPostPromoCreateEndpoint(svc), botAuth)
	e.GET(apilib.GetBotPromos, bot.NewGetPromosForBotEndpoint(svc), botAuth)
}
