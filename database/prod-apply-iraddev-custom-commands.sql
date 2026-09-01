-- Moves iRadDev's static link/blurb chat commands off the Godot overlay and
-- into peanutbudderbot as channel-scoped custom commands.
--
-- These are the commands whose entire behaviour is "print one fixed line in
-- chat". They currently live in ridiculous-stream-overlay
-- (classes/RSCustom.gd, registered in add_commands()), which means they only
-- answer while the overlay is running. As custom rows the bot answers them
-- whenever it is in the channel, and the overlay can drop the handlers.
--
-- Safe to re-run: ON CONFLICT DO NOTHING against command_channel_trigger_uk
-- (channel_twitch_id, trigger) WHERE scope = 'channel'.
--
-- Deploy: no code change needed. Custom commands carry a response template
-- rather than a handler_key, so nothing has to exist in the bot's
-- builtinRegistry and no binary has to ship first. The bot picks a channel's
-- custom set up on its next go-live (loadCustomCommands, fired by
-- stream.online), so these go live on iRadDev's next stream, not instantly.
--
-- After applying, delete the matching _cmd() registrations and their funcs
-- from RSCustom.gd, or chat gets two replies to every trigger.

INSERT INTO nivek.command
    (trigger, kind, response_tmpl, description, scope, channel_twitch_id, enabled, min_role)
VALUES
    ('!wishlist', 'custom',
     'Check out our game on Steam! Typing Tower Defence Game -> A Few of US: Operation Nightshade https://s.team/a/4326920 . Follow IrishJohnGames curated list of steam games HERE: https://store.steampowered.com/curator/45553695-Games-Developed-Live-by-Streamers/ <- DO IT!',
     'Steam link for A Few of US: Operation Nightshade, plus the IrishJohnGames streamer-built curator list.',
     'channel', '443367221', TRUE, 'everyone'),

    ('!steam', 'custom',
     'Check out our game on Steam! Typing Tower Defence Game -> A Few of US: Operation Nightshade https://s.team/a/4326920 . Follow IrishJohnGames curated list of steam games HERE: https://store.steampowered.com/curator/45553695-Games-Developed-Live-by-Streamers/ <- DO IT!',
     'Alias of !wishlist — the Steam page for A Few of US: Operation Nightshade.',
     'channel', '443367221', TRUE, 'everyone'),

    ('!itch', 'custom',
     'Check out our game on itch.io! Typing Tower Defence Game -> A Few of US: Operation Nightshade https://vhoyer.itch.io/operation-nightshade .',
     'itch.io page for A Few of US: Operation Nightshade.',
     'channel', '443367221', TRUE, 'everyone'),

    ('!jam', 'custom',
     'Not doing any jam now after winning this one! https://itch.io/jam/jern-jam-2025 . Also go check trickylady and camsdono game jam game I helped with: https://trickylady.itch.io/bam',
     'Jern Jam 2025 result, and the jam game built with trickylady and camsdono.',
     'channel', '443367221', TRUE, 'everyone'),

    ('!commands', 'custom',
     'Use a combination of ![command] for chat: hl (highlight), hd(hidden), rb(rainbow), big, small, wave, pulse, tornado, shake',
     'Lists the chat text effects the overlay understands (highlight, hidden, rainbow, big, small, wave, pulse, tornado, shake).',
     'channel', '443367221', TRUE, 'everyone')
ON CONFLICT (channel_twitch_id, trigger) WHERE scope = 'channel' DO NOTHING;


-- ---------------------------------------------------------------------------
-- HELD BACK: !discord and !c_source
--
-- These two also print a fixed line, but they are not pure chat replies — each
-- plays a sound on the streaming PC as well (RS.play_sfx("discord") and the
-- sfx_scissors.ogg one-shot in c_source()). Moving them now would keep the
-- text and silently lose the sound, because nothing carries a command from the
-- bot down to the overlay yet.
--
-- Uncomment once the relay grows a 'command' event kind, or now if you would
-- rather have the links answer while the overlay is closed and accept losing
-- the sfx. If you uncomment, remove the matching handlers from RSCustom.gd.
-- ---------------------------------------------------------------------------

-- INSERT INTO nivek.command
--     (trigger, kind, response_tmpl, description, scope, channel_twitch_id, enabled, min_role)
-- VALUES
--     ('!discord', 'custom',
--      'Join Discord: https://discord.gg/4YhKaHkcMb',
--      'Invite link for the Ridiculous Stream Discord.',
--      'channel', '443367221', TRUE, 'everyone'),
--
--     ('!c_source', 'custom',
--      'Check out Trickylady first published game with finisfine special guest: https://trickylady.itch.io/the-exam-scam',
--      'Link to The Exam Scam by Trickylady, featuring finisfine.',
--      'channel', '443367221', TRUE, 'everyone')
-- ON CONFLICT (channel_twitch_id, trigger) WHERE scope = 'channel' DO NOTHING;
