package api

const (
	HelloWorld = "/"

	// Twitch OAuth — see endpoints/user/auth/twitch.go. /start kicks off the
	// authorize redirect; /callback is what Twitch sends the user back to.
	GetTwitchStart    = "/auth/twitch/start"
	GetTwitchCallback = "/auth/twitch/callback"

	//
	// Twitch Webhook Subscription Callback
	//

	TwitchWebhookSubscriptionRequest = "/eventsub"

	//
	// secure routes
	//

	PostLogout         = "/logout"
	PostFetchUserData  = "/profile"
	GetUserTasks       = "/user/:id/task"
	PostCreateUserTask = "/user/:id/task"
	GetFishingScore    = "/fishing"

	PostWeather = "/weather"

	GetAutoShoutChatters       = "/auto-shout"
	PostCreateAutoShoutChatter = "/auto-shout"
	PostUpdateAutoShoutChatter = "/auto-shout/:id"
	DeleteAutoShoutChatter     = "/auto-shout/:id"

	PostCreateMessage = "/message"
	GetMessages       = "/message"

	// DF dashboard (public, no auth — dashboard is publicly fanned out)
	GetDFSnapshot = "/df/snapshot"
	// DF dashboard ingest from the DFHost pusher (HMAC-authed in handler)
	PostDFSnapshot = "/df/snapshot"

	// twitch-bot RPC — Pi calls these instead of touching Postgres directly.
	// HMAC-authed (BOT_API_HMAC_KEY) via the shared HMACMiddleware.
	GetBotChannels        = "/bot/channels"
	GetActiveChannels     = "/bot/channels/active"
	PostBotBreadIncrement = "/bot/bread/increment"
	GetBotBreadTotal      = "/bot/bread/total"
	PostBotLurkMessage    = "/bot/lurk/message"
	PostBotFishGo         = "/bot/fish/go"
	PutBroadcasterState   = "/bot/channels/putstate"
)
