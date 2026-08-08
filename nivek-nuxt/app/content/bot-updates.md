_Latest first._

### 2026-08-08 - Twitch API Integrations

My last devlog stated that I had trouble getting people on board with this given
the fact that my understanding of the proper flow required them to visit the bot
website and perform the oauth flow. This definitely would easily come off as some
kind of phishing attempt or maybe credential harvesting. To add to the mess, I had
an undetected bug that actually prevented new users from logging in successfully.
A one-two punch that completely prevented me from expanding the bot.

I've since discovered that twitch's get-users endpoint, the '/helix/users' endpoint,
actually has two different flows available. One of which does ride off of the oauth
flow - this is a sort of "who is the user that owns this token" flow. The other rides
off of a twitch "broadcaster id" (which is a publically exposed userID type of value)
or a "twitch login" (which is another publically exposed value - this is a normalized
string: entirely lowercase with whitespaces removed). This second flow allows me to
submit one of those values, and get the other as well as the user's display name.

Now, the library I am using to connect to twitch's IRC has a lossy approach to fetching
user twitch_login values. I figure this is likely not going to be an issue, so I've
opted to use it anyways. The broadcaster ID however is NOT lossy, and if I ever run
into issues with the login value, I can use the ID to resolve that. An issue for
future me, potentially.

Given that both of these values are available publically, all I need to do is either
guess someone's login, manually insert it into the DB, and reboot the bot. Or, I
could build out a !joinme command.

So now we have a !joinme command. This allows the bot to join your channel from
another channel. It does not have to be run from my channel, it can be run from
anywhere. Hopefully this leads to explosive growth?

Now for two of probably the hardest issues with this project:
1) I need to make the bot _useful_. Currently it is mostly just a silly !fish and
!dad bot. It serves no actual utility to the streamer. I could go towards a moderator
approach with this and have it act like Sery_bot and ban spammers, or I could go a
more Moobot approach and let it manipulate stream information (title, category, etc).
Neither of these options really entertain me, and this is a hobby project. It must
remain interesting for me to keep developing it. A higher usercount is what interests
me, but for that I probably need a reason for people to want it. !dad jokes can only
go so far, and not everyone has the same sense of humor.
2) Find a way to monetize the bot. Obviously I'm not going to add advertisements to
chats, and I very much dislike advertisements on the website. Streamers seem to
only visit the website when they need to, so the actual site likey won't be very
high traffic. Ads very much degrade the experience of a website, so I very much dislike
this option. I think StreamElements did some kind of partnerships with brands, but
I don't know how to establish that kind of relationship. I am a programmer, not a
marketer or salesman.

I'm definitely not breaking my back to try to monetize this right now. Any approach
likely won't be effective with my 6 or 7 users. I would need a much higher usercount
to make that a viable option. I'm more interested in experiencing managing this system
by myself under high traffic. It is fun to write the code and consider what approach
is the most reasonable for the task at hand, and learning about various aspects of
software engineering along the way: programming, hosting, alerting, integrations, etc.

I should probably start with some kind of visibility tool so users can see all available
commands, and allowing users to create their own commands might be the best way forwards.

Of course the website's UI can (and probably will always) use some improvements. Stay tuned!

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

