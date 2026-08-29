package twitchbot

import "log"

// commandsPageURL is the public, no-login page that lists every command and
// action the bot supports. !pbcommands just points chat at it rather than
// dumping the whole list inline. It's the bot's own domain, so it won't trip
// link-spam heuristics — the same pattern the !DF welcome and !joinme promo
// messages already use.
const commandsPageURL = "https://peanutbudderbot.com/commands"

// handlePbCommandsCommand replies with a link to the public commands page.
func (b *Bot) handlePbCommandsCommand(message *chatMessageEvent) {
	b.say(message.BroadcasterUserId, "📜 All my commands and actions are listed here: "+commandsPageURL)
	log.Printf("[PBCOMMANDS] [%s] %s", message.BroadcasterUserLogin, message.ChatterUserLogin)
}
