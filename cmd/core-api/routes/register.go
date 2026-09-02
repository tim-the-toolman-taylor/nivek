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
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/overlay"
	promoEp "github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/promo"
	stalkEp "github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/stalk"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/task"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/user/auth"
	"github.com/tim-the-toolman-taylor/nivek/cmd/core-api/endpoints/weather"
	apilib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/api"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivekmiddleware"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
)

func RegisterRoutes(svc nivek.NivekService, e *echo.Group) {
	// Overlay relay. The registry is process-local shared state: the EventSub
	// webhook writes to it and the websocket endpoint reads from it, so every
	// overlay route must be handed the same instance.
	overlayRelay := overlayrelay.NewService(svc)
	overlayRegistry := overlayrelay.NewRegistry()

	e.GET(apilib.HelloWorld, endpoints.NewIndexEndpoint(svc))

	// OAuth endpoints are public. They create the local HttpOnly session cookie
	// only after state validation and a successful Twitch identity lookup.
	e.GET(apilib.GetTwitchStart, auth.NewTwitchStartEndpoint(svc))
	e.GET(apilib.GetTwitchCallback, auth.NewTwitchCallbackEndpoint(svc))

	// Logout must remain callable with an expired or corrupt JWT so the browser
	// can always clear stale cookies.
	e.POST(apilib.PostLogout, auth.NewLogoutEndpoint(svc))

	// Shared by website + bot. Global Commands are publicly accessible, I can't think
	// of a good reason to gate the bot's fetch behind a privkey while letting an identical
	// one run without any auth. Either both need auth or neither, and I think neither is
	// the proper handling for this
	e.GET(apilib.GetGlobalEnabledCommands, commands.NewGetCommandsEndpoint(svc))

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

	e.GET(apilib.GetDadResponses, dad.NewGetDadResponsesEndpoint(svc), authenticated)
	e.POST(apilib.PostCreateDadResponse, dad.NewCreateDadResponseEndpoint(svc), authenticated)
	e.DELETE(apilib.DeleteDadResponse, dad.NewDeleteDadResponseEndpoint(svc), authenticated)

	e.GET(apilib.GetPromos, promoEp.NewGetPromosEndpoint(svc), authenticated)
	e.POST(apilib.PostCreatePromo, promoEp.NewCreatePromoEndpoint(svc), authenticated)
	e.POST(apilib.PostUpdatePromo, promoEp.NewUpdatePromoEndpoint(svc), authenticated)
	e.DELETE(apilib.DeletePromo, promoEp.NewDeletePromoEndpoint(svc), authenticated)

	e.GET(apilib.GetOverlayDevices, overlay.NewListDevicesEndpoint(svc, overlayRelay, overlayRegistry), authenticated)
	e.POST(apilib.PostCreateOverlayDevice, overlay.NewCreateDeviceEndpoint(svc, overlayRelay, overlayRegistry), authenticated)
	e.DELETE(apilib.DeleteOverlayDevice, overlay.NewRevokeDeviceEndpoint(svc, overlayRelay, overlayRegistry), authenticated)
	e.GET(apilib.GetOverlayDownload, overlay.NewDownloadEndpoint(svc), authenticated)

	e.GET(apilib.GetStalk, stalkEp.NewGetStalkEndpoint(svc), authenticated)
	e.POST(apilib.PostStalk, stalkEp.NewSetStalkEndpoint(svc), authenticated)
	e.DELETE(apilib.DeleteStalk, stalkEp.NewClearStalkEndpoint(svc), authenticated)

	// Public DF dashboard and HMAC-authenticated ingest.
	e.GET(apilib.GetDFSnapshot, df.NewGetSnapshotEndpoint(svc))
	e.POST(apilib.PostDFSnapshot, df.NewPostSnapshotEndpoint(svc))

	// Public: Twitch authenticates by signing the message, not by session, so
	// this route stays outside the JWT middleware and the credentialed CORS
	// policy.
	e.POST(apilib.PostOverlayEventSub, overlay.NewEventSubEndpoint(svc, overlayRelay, overlayRegistry))
	// Public: the caller is a viewer's browser posting a Twitch-signed Bits
	// receipt, not a session. Its cross-origin browser POST needs the extension
	// origin allowed in the CORS policy (opened in main.go when configured).
	e.POST(apilib.PostOverlayExtension, overlay.NewExtensionEndpoint(svc, overlayRelay, overlayRegistry))
	// Authenticated by device token inside the websocket handshake rather than
	// by session cookie: the client is a desktop app, and this keeps the
	// credential out of proxy access logs.
	e.GET(apilib.GetOverlayConnect, overlay.NewConnectEndpoint(svc, overlayRelay, overlayRegistry))

	botAuth := nivekmiddleware.NewHMACMiddleware("BOT_API_HMAC_KEY")
	e.GET(apilib.GetActiveChannels, bot.NewGetActiveChannelsEndpoint(svc), botAuth)
	e.POST(apilib.PostHealLegacyUser, bot.NewPostHealLegacyUserEndpoint(svc), botAuth)
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
	e.GET(apilib.GetBotChannelCommands, bot.NewGetChannelCommands(svc), botAuth)
	e.GET(apilib.GetBotAutoShouters, bot.NewGetAutoShoutChatters(svc), botAuth)
	e.POST(apilib.PostBotAutoShoutIncrement, bot.NewPostAutoShoutIncrement(svc), botAuth)
	e.POST(apilib.PostBotPromoCreate, bot.NewPostPromoCreateEndpoint(svc), botAuth)
	e.GET(apilib.GetBotPromos, bot.NewGetPromosForBotEndpoint(svc), botAuth)
	e.POST(apilib.PostBotPromoEditLast, bot.NewPostPromoEditLastEndpoint(svc), botAuth)
	e.POST(apilib.PostBotPromoDeleteLast, bot.NewPostPromoDeleteLastEndpoint(svc), botAuth)
	e.GET(apilib.GetBotStalkTarget, bot.NewGetStalkTargetEndpoint(svc), botAuth)
	e.POST(apilib.PostBotStalkSet, bot.NewPostStalkSetEndpoint(svc), botAuth)
	e.POST(apilib.PostBotStalkClear, bot.NewPostStalkClearEndpoint(svc), botAuth)
	e.POST(apilib.PostBotStalkLastMessage, bot.NewPostStalkLastMessageEndpoint(svc), botAuth)
	// Overlay command dispatch. botAuth, not a session: this writes to a user's
	// overlay event log on the bot's say-so.
	e.POST(apilib.PostBotOverlayDispatch, overlay.NewDispatchEndpoint(svc, overlayRelay, overlayRegistry), botAuth)
}
