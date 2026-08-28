-- Create a schema for the project
create SCHEMA IF NOT EXISTS nivek;

-- Application-level table
-- contains information unique to the bot itself
create TABLE IF NOT EXISTS nivek.app (
    id SERIAL PRIMARY KEY,
    last_wiped_at TIMESTAMP
);

-- Create users table
-- this table will represent every channel the twitch bot should join. The "users" of the twitch bot.
-- New rows are created via Twitch OAuth (twitch_id is canonical). The legacy
-- email/password columns are kept nullable so pre-OAuth rows continue to load,
-- but no new flow writes to them.
CREATE TABLE IF NOT EXISTS nivek.users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    twitch_id VARCHAR(64) UNIQUE,
    twitch_login VARCHAR(50),
    twitch_display_name VARCHAR(100),
    bot_opt_in BOOLEAN NOT NULL DEFAULT FALSE,
    is_live BOOLEAN NOT NULL DEFAULT FALSE,
    -- Per-stream key (our own UUID), minted on each go-live. Used to tell which
    -- broadcast is currently live so auto-shout de-dup survives a bot restart.
    stream_key TEXT
);

CREATE TABLE IF NOT EXISTS nivek.command (
    id SERIAL PRIMARY KEY,

    -- identity / dispatch
    trigger       TEXT NOT NULL,
    kind          TEXT NOT NULL DEFAULT 'builtin'
                    CHECK (kind in ('builtin', 'custom')),
    handler_key   TEXT, -- require iff kind='builtin'
    response_tmpl TEXT, -- template - required iff kind='custom'
    description   TEXT NOT NULL DEFAULT '', -- description for the /commands page on the website

    -- scope (hook for per-channel CUSTOM commands later; every row is 'global' for now)
    scope             TEXT NOT NULL DEFAULT 'global'
                        CHECK (scope IN ('global', 'channel')),
    channel_twitch_id VARCHAR(64), -- NULL for global; set for channel-custom

    -- DEFAULT behavior - will be used for per-channel-command settings (future table: channel_command_settings)
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    min_role      TEXT    NOT NULL DEFAULT 'everyone'
                    CHECK (min_role IN ('everyone', 'sub', 'vip', 'mod', 'broadcaster')),
    cooldown_secs INTEGER NOT NULL DEFAULT 0 CHECK (cooldown_secs >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- builtins carry a handler key; customs carry a template
    CONSTRAINT commands_kind_payload CHECK (
        (kind = 'builtin' AND handler_key IS NOT NULL AND response_tmpl IS NULL) OR
        (kind = 'custom'  AND response_tmpl IS NOT NULL AND handler_key IS NULL)
    ),
    -- global rows are channel-less; channel rows name a channel
    CONSTRAINT commands_scope_channel CHECK (
        (scope = 'global'  AND channel_twitch_id IS NULL) OR
        (scope = 'channel' AND channel_twitch_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS command_global_trigger_uk
  ON nivek.command (trigger) WHERE scope = 'global';
CREATE UNIQUE INDEX IF NOT EXISTS command_channel_trigger_uk
  ON nivek.command (channel_twitch_id, trigger) WHERE scope = 'channel';

-- Seed the global built-ins. handler_key MUST match a key in the bot's
-- builtinRegistry (internal/libraries/twitchbot/bot.go) — the bot links each
-- trigger to its Go handler via this string, and an unknown key fails the bot
-- at boot. ON CONFLICT keeps this idempotent against the unique trigger index.
INSERT INTO nivek.command (trigger, kind, handler_key, min_role, description) VALUES
    ('!dad',        'builtin', 'dad_roll',    'everyone', 'Rolls a random dad joke. Rate-limited per stream so it stays fun, not spammy.'),
    ('!fish',       'builtin', 'fish',        'everyone', 'Cast a line and reel in a random catch — builds up your fishing score over time.'),
    ('!bread',      'builtin', 'bread',       'everyone', 'Bump the channels communal bread counter. 🍞'),
    ('!lurk',       'builtin', 'lurk',        'everyone', 'Let the streamer know youre sticking around in the background.'),
    ('!joinme',     'builtin', 'join_me',     'everyone', 'Have the bot join YOUR channel. In channels other than the bots home chats, mention the bot: @peanutbudderbot !joinme'),
    ('!banish',     'builtin', 'banish',      'mod',      'Remove the bot from your channel (kept your data — just opts out). Broadcaster or moderator only.'),
    ('!pbcommands', 'builtin', 'pb_commands', 'everyone', 'Drops a link to this very page in chat.'),
    ('!newpromo',   'builtin', 'new_promo',   'mod',      'Set a recurring message the bot re-posts while youre live (broadcaster/mods only). Create one with "!newpromo 30m <message>". Replace your most recent message with "!newpromo edit-last <interval> <new message>" (e.g. !newpromo edit-last 20m New text here), or remove it with "!newpromo delete-last". Full management is on the dashboard.'),
    ('!stalk',      'builtin', 'stalk',       'everyone', 'Quotes the last chat message from the chatter this channel is stalking. Anyone can run it; mods/broadcaster pick the target with "!stalk set <username>" and clear it with "!stalk clear".')
ON CONFLICT (trigger) WHERE scope = 'global' DO NOTHING;

CREATE TABLE IF NOT EXISTS nivek.channel_command_settings (
    channel_twitch_id VARCHAR(64) NOT NULL,
    command_id        INTEGER NOT NULL REFERENCES nivek.command(id) ON DELETE CASCADE,
    enabled           BOOLEAN, -- NULL = inherit command.enabled
    min_role          TEXT,    -- NULL = inherit command.min_role
    cooldown_secs     INTEGER, -- NULL = inherit command.cooldown_secs
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_twitch_id, command_id)
);

CREATE INDEX IF NOT EXISTS channel_command_settings_command_id_idx
  ON nivek.channel_command_settings (command_id);

-- Recurring custom messages ("promos" / timers). Each row is one message the bot
-- re-posts in `channelname` (a lowercased Twitch login) every `interval_seconds`
-- while that channel is live. Broadcasters/mods create them from chat with
-- `!newpromo <interval> <message>`, or manage them from the dashboard. A channel
-- may have any number of promos.
CREATE TABLE IF NOT EXISTS nivek.promo (
    id SERIAL PRIMARY KEY,
    channelname      VARCHAR(50) NOT NULL,
    message          TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL DEFAULT 1800
                       CHECK (interval_seconds >= 60 AND interval_seconds <= 86400),
    enabled          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS promo_channelname_idx ON nivek.promo (channelname);

-- Per-channel !stalk target. One row per channel (the chatter currently being
-- stalked). Mods/broadcaster set it from chat with `!stalk set <username>`;
-- anyone can then run `!stalk` to quote that chatter's last message. Last-message
-- lookup itself is in-memory on the bot (not stored here).
CREATE TABLE IF NOT EXISTS nivek.stalk (
    id SERIAL PRIMARY KEY,
    channelname  VARCHAR(50) NOT NULL,
    target_login VARCHAR(50) NOT NULL,
    set_by       VARCHAR(50),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT stalk_channel_uk UNIQUE (channelname)
);

-- Migrate the bot's original hardcoded self-promo (previously a Go ticker in
-- twitchbot.runPromotionMessageLoop) into a normal promo row so every recurring
-- message flows through one path. Idempotent on the leading message text.
INSERT INTO nivek.promo (channelname, message, interval_seconds)
SELECT 'timallenfanclubofficial',
       'Test out my bot! Use !joinme to have it join your channel. Get rid of it with !banish, and check out https://peanutbudderbot.com for configuration options. The bot is open source, so feel free to contribute, fork, or just learn about it at https://github.com/debugging-in-prod/nivek',
       1800
WHERE NOT EXISTS (
    SELECT 1 FROM nivek.promo p
    WHERE p.channelname = 'timallenfanclubofficial'
      AND p.message LIKE 'Test out my bot!%'
);


-- Idempotent upgrades for any existing database that pre-dates the Twitch
-- OAuth migration. `IF NOT EXISTS` keeps re-runs safe and lets older rows stay
-- valid with NULL twitch_* values until the user signs in via Twitch again.
ALTER TABLE nivek.users ALTER COLUMN username DROP NOT NULL;
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS twitch_id VARCHAR(64) UNIQUE;
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS twitch_login VARCHAR(50);
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS stream_key TEXT;
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS twitch_display_name VARCHAR(100);
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS bot_opt_in BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE nivek.users ADD COLUMN IF NOT EXISTS is_live BOOLEAN NOT NULL DEFAULT FALSE;

-- Table to track fishing scores per user
create TABLE IF NOT EXISTS nivek.fish_score (
    id SERIAL PRIMARY KEY,
    channelname VARCHAR(255) NOT NULL,
    chattername VARCHAR(255) NOT NULL,
    score INT NOT NULL DEFAULT 0,
    fish JSONB NOT NULL DEFAULT '[]',
    trash_caught INT NOT NULL DEFAULT 0,
    times_fished INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Composite unique constraint
    CONSTRAINT unique_user_chatter UNIQUE (channelname, chattername)
);

-- Table to track bread counts
create TABLE IF NOT EXISTS nivek.bread (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    chattername VARCHAR(255) NOT NULL,
    count INT DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Composite unique constraint
    CONSTRAINT unique_user_chatter UNIQUE (user_id, chattername)
);

-- Auto shoutout whitelist system
CREATE TABLE IF NOT EXISTS nivek.auto_shout (
    id SERIAL PRIMARY KEY,
    chattername VARCHAR(255) NOT NULL,
    shout_count int NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    -- Broadcaster's Twitch user id. Replaced the original channelname column;
    -- the bot fetches a channel's shoutout list by broadcaster id on go-live.
    twitch_id INTEGER NOT NULL DEFAULT 0,
    -- The users.stream_key value at the moment this chatter was last shouted.
    -- Compared against the broadcaster's current stream_key to skip re-shouting
    -- within the same stream (survives a bot restart).
    stream_key TEXT
);
ALTER TABLE nivek.auto_shout ADD COLUMN IF NOT EXISTS stream_key TEXT;

-- Per-stream, per-chatter !dad roll tally. roll_count is scoped to the stream
-- identified by stream_key (the users.stream_key value stamped when the count was
-- last touched), so a new go-live resets everyone's allotment. Persisted so the
-- bot's in-memory rate-limit counters survive a restart mid-stream.
CREATE TABLE IF NOT EXISTS nivek.dad_usage (
    id SERIAL PRIMARY KEY,
    twitch_id INTEGER NOT NULL,
    chattername TEXT NOT NULL,
    roll_count INTEGER NOT NULL DEFAULT 0,
    stream_key TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT dad_usage_twitch_chatter_unique
    UNIQUE (twitch_id, chattername)
);

CREATE TABLE IF NOT EXISTS nivek.bread (
    id SERIAL PRIMARY KEY,
    channelname VARCHAR(255) NOT NULL,
    chattername VARCHAR(255) NOT NULL,
    bread_count int NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT bread_channel_chatter_unique
    UNIQUE (channelname, chattername)
);

CREATE TABLE lurk (
    id SERIAL PRIMARY KEY,
    channelname   TEXT NOT NULL,
    chattername   TEXT NOT NULL,
    lurk_count    INTEGER NOT NULL DEFAULT 0,
    updated TIMESTAMP NOT NULL DEFAULT NOW(),
    created TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Composite unique constraint
    CONSTRAINT unique_lurker UNIQUE (channelname, chattername)
);

-- !dad responses. Global rows (is_global = TRUE, channelname NULL) are the shared
-- defaults available in every channel; per-channel rows are that channel's own
-- additions. A !dad roll picks randomly from (globals + the channel's own rows).
CREATE TABLE IF NOT EXISTS dad_response (
    id SERIAL PRIMARY KEY,
    channelname VARCHAR(50),
    response TEXT NOT NULL,
    is_global BOOLEAN NOT NULL DEFAULT FALSE,
    use_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- One response per channel; globals are unique on the text alone.
CREATE UNIQUE INDEX IF NOT EXISTS dad_response_channel_uniq
    ON dad_response (channelname, response) WHERE NOT is_global;
CREATE UNIQUE INDEX IF NOT EXISTS dad_response_global_uniq
    ON dad_response (response) WHERE is_global;

-- Seed the original hardcoded !dad lines as shared global defaults (idempotent).
INSERT INTO dad_response (channelname, response, is_global)
SELECT NULL, v.response, TRUE
FROM (VALUES
    ('still out getting milk!'),
    ('I need you to blow in this breathalyzer so we can go back to your moms house'),
    ('wrong kid died')
) AS v(response)
WHERE NOT EXISTS (
    SELECT 1 FROM dad_response d WHERE d.is_global AND d.response = v.response
);
