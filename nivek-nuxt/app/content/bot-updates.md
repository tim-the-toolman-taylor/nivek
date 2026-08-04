_Latest first._

### 2026-08-04 - Webhooks!

First off - what are webhooks? Webhooks are an "event" that is propagated by twitch.
They are not unique to twitch - lots of other systems use this, some of which are
payment systems like Stripe, and Paypal. E-commerce uses these as well so Shopify,
Bigcommerce. Developer tools such as Github and Bitbucket use these as well, and
also chat platforms like Slack and Discord. They are not uncommon in the world of
web-based programming.

Now what do they do? As previously stated, they are "events" emitted by a source.
These "events" can then be used to trigger logic in a program. If you've ever seen
a Discord bot post a message when someone goes live, that is most likely a Discord
bot ingesting a webhook from Twitch. Think of it like this: "Twitch sends out a
message the X is live, and things listening for that message respond accordingly".

Now why did I setup webhooks for this chatbot?

The bot was originally running off of a "once you opt in, the bot never leaves your
chat" design. This was great for initial testing, but is not a long-term solution.

The most common issue I was finding was: when a streamer goes offline, the chat can
be idle for long periods of time. This idle-period tended to cause the bot's IRC
connection to drop, and would require me to manually reboot it to bring it back. I
ended up implementing an "idle reconnect" system where the bot automatically pings
twitch's IRC channel to stay alive or reconnect if it needs to, but if I were to
have >100,000 streamers use this bot, that would be >100,000 connections to maintain
around the clock. Not a scalable solution and therefore not a long-term solution.

Another issue was with the "auto-shout" command. "auto-shout" is a system where you
can add your friends to a list, and when they put their first message in chat after
you go live, the bot responds with a "!so @<username>" message. This message prompts
a separate bot (commonly streamelements from my experience) to pull whatever game
this chatter was last playing on stream, and tells your other viewers to check out
their channel. So, its a tool to "shoutout" your other streamer buddies.

Now the issue - with the bot staying in your chat 24/7 regardless of if you are
live or not, the bot had trouble keeping track of who recieved a shoutout and who
did not. I had the bot keeping lists of users in-memory 24/7, and the only way to
get a second shoutout was when 24 hours passed from your first shoutout. Of course,
this 24 hour timer would not persist properly on reboot, and this ended up being a
very noisy and very broken system.

So how do webhooks fix all this?

First of all, the bot only joins chats when you go live. I don't need a "stay-alive"
ping, because your chat is much more likely to be active when you're actively streaming.
Second, if the bot is aware of when you go live, then the bot can simply "pull" your
auto-shout list, and pluck names out as they recieve their shoutout. Much more simple!

Now the hard part?

In order to get webhooks registered for people, I'm having to ask them to visit this
site and login with twitch. This requires them to either trust me (trust me bro I'm
a stranger on the internet), or to understand how an OAuth flow works. I can't be
upset with people for being careful on the internet - I support any and all efforts
to maintain digital privacy rights - but I am not sure how much more transparent I
can be by making the code that performs these actions publically available. Small
streamers think I am doing something nefarious and might conjure up a reason for this
to be unsafe, but they don't conjure up the eyeballs to read the code. This is a much
harder problem than webhooks!

### 2026-06-08 — First public smoke test - Twitch Plays Dwarf Fortress

The Twitch-plays-DF bot is online and listening in
[twitch.tv/timallenfanclubofficial](https://twitch.tv/timallenfanclubofficial).
Expect rough edges: most verbs work, some will silently fail or hit
"not supported yet." Please chat what you tried and what happened.

This page is the source of truth for what's available. If the bot
responds to a command that isn't documented here, that's a doc bug —
please flag it.

For a bit of context - "Twitch Plays" started as a social experiment Feb 12, 2014
that was first started by the twitch channel "Twitch Plays Pokemon". The original
project allowed chatters to play pokemon by simply writing in chat. The phrase "up"
would move the character "up" one tile, "A" would perform the equivalent of pressing
"A" on a gameboy, and "B", "left", and so on.

The result? Gunniess World Record in viewership on its first run - 36 million views.
The channel continues to stream this project today, although it has fallen off in
popularity quite a bit. I think it is an interesting engineering challenge to wire this
together, to keep it responsive enough for it to be engaging, and to handle the
volume of traffic that comes with potentially 36 million people chatting at the same
time.

