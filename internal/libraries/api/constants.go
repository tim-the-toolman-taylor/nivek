package api

const (
	HelloWorld = "/"

	GetTwitchStart    = "/auth/twitch/start"
	GetTwitchCallback = "/auth/twitch/callback"

	TwitchWebhookSubscriptionRequest = "/eventsub"

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

	PostCreateMessage = "/message"
	GetMessages       = "/message"

	GetDadResponses       = "/dad"
	PostCreateDadResponse = "/dad"
	DeleteDadResponse     = "/dad/:id"

	GetDFSnapshot  = "/df/snapshot"
	PostDFSnapshot = "/df/snapshot"

	PutCreateNewUser      = "/bot/channels/create"
	GetActiveChannels     = "/bot/channels/active"
	PostHealLegacyUser    = "/bot/channels/heal"
	PostBotBreadIncrement = "/bot/bread/increment"
	GetBotBreadTotal      = "/bot/bread/total"
	PostBotLurkMessage    = "/bot/lurk/message"
	PostBotFishGo         = "/bot/fish/go"
	PutBroadcasterState   = "/bot/channels/putstate"
	PostBotOptOut         = "/bot/channels/optout"
	PostBotOptInCheck     = "/bot/channels/optin"
	PostBotDadRandom      = "/bot/dad"
	PostBotDadAdd         = "/bot/dad/add"
	PostBotDadRemove      = "/bot/dad/remove"

	PostBotJoinChannel = "/internal/join"
)

// PostFetchUserData remains for downstream callers during the GET /profile
// migration. New code should use GetUserProfile.
const PostFetchUserData = GetUserProfile
