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
    username VARCHAR(50) UNIQUE,
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
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Composite unique constraint
    CONSTRAINT unique_lurker UNIQUE (channelname, chattername)
);

CREATE TABLE message (
    id SERIAL PRIMARY KEY,
    sender VARCHAR(255) NOT NULL,
    message varchar(255) NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
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
