package api

const (
	HelloWorld = "/"

	GetTwitchStart    = "/auth/twitch/start"
	GetTwitchCallback = "/auth/twitch/callback"

	TwitchWebhookSubscriptionRequest = "/eventsub"

	GetGlobalEnabledCommands = "/commands"

	PostLogout         = "/logout"
	GetUserProfile     = "/profile"
	GetUserTasks       = "/user/:id/task"
	PostCreateUserTask = "/user/:id/task"
	GetFishingScore    = "/fishing"

	PostWeather = "/weather"

	GetAutoShoutChatters       = "/auto-shout"
	PostCreateAutoShoutChatter = "/auto-shout"
	PostUpdateAutoShoutChatter = "/auto-shout/:id"
	DeleteAutoShoutChatter     = "/auto-shout/:id"

	GetDadResponses       = "/dad"
	PostCreateDadResponse = "/dad"
	DeleteDadResponse     = "/dad/:id"

	// Authed (dashboard) recurring-message CRUD. GET/POST share "/promo" and
	// POST/DELETE share "/promo/:id" — distinct HTTP methods, so Echo does not
	// treat them as duplicate routes.
	GetPromos       = "/promo"
	PostCreatePromo = "/promo"
	PostUpdatePromo = "/promo/:id"
	DeletePromo     = "/promo/:id"

	GetDFSnapshot  = "/df/snapshot"
	PostDFSnapshot = "/df/snapshot"

	PutCreateNewUser          = "/bot/channels/create"
	GetActiveChannels         = "/bot/channels/active"
	PostHealLegacyUser        = "/bot/channels/heal"
	PostBotBreadIncrement     = "/bot/bread/increment"
	GetBotBreadTotal          = "/bot/bread/total"
	PostBotLurkMessage        = "/bot/lurk/message"
	PostBotFishGo             = "/bot/fish/go"
	PutBroadcasterState       = "/bot/channels/putstate"
	PostBotOptOut             = "/bot/channels/optout"
	PostBotOptInCheck         = "/bot/channels/optin"
	PostBotDadRandom          = "/bot/dad"
	PostBotDadAdd             = "/bot/dad/add"
	PostBotDadRemove          = "/bot/dad/remove"
	PostBotDadUsage           = "/bot/dad/usage"
	PostBotDadIncrement       = "/bot/dad/increment"
	GetBotAutoShouters        = "/bot/autoshout/:bid"
	PostBotAutoShoutIncrement = "/bot/autoshout/increment"
	PostBotPromoCreate        = "/bot/promo"
	GetBotPromos              = "/bot/promos"
	PostBotPromoEditLast      = "/bot/promo/edit-last"
	PostBotPromoDeleteLast    = "/bot/promo/delete-last"

	PostBotJoinChannel = "/internal/join"
)
