-- Overlay commands: the global builtin set that any channel with a paired
-- overlay inherits. Design and rationale in docs/OVERLAY_COMMANDS.md.
--
-- ###########################################################################
-- ## DO NOT APPLY THE INSERT BEFORE THE DISPATCH CODE SHIPS.               ##
-- ###########################################################################
--
-- Two independent reasons, either one of which bites:
--
--   1. getGlobalEnabledCommands (internal/libraries/twitchbot/bot.go) returns
--      an error for a handler_key that is not in builtinRegistry, and that
--      error fails the bot at boot. Inserting these rows while the current
--      binary is deployed crashes the bot on its next restart -- the same trap
--      called out in prod-apply-stalk-builtin.sql.
--
--   2. Nothing reads `requires` yet. b.commands is one map consulted for every
--      channel, so until dispatch honours the capability gate these commands
--      fire in EVERY channel the bot sits in, including the ~all of them with
--      no overlay attached.
--
-- Deploy order:
--   1. Run the ALTER below. Safe at any time -- adding a nullable column that
--      nothing reads changes no behaviour, and every existing row keeps
--      requires = NULL (unconditional).
--   2. Ship the bot with the overlay_* handlers registered in builtinRegistry
--      AND with the `requires` gate honoured at dispatch (including the
--      fall-through to custom commands on an unmet requirement -- see
--      docs/OVERLAY_COMMANDS.md, "The fall-through").
--   3. Then run the INSERT.
--
-- Safe to re-run: ADD COLUMN IF NOT EXISTS, INSERT ON CONFLICT DO NOTHING.


-- Step 1 -- safe now.
ALTER TABLE nivek.command
    ADD COLUMN IF NOT EXISTS requires TEXT
        CHECK (requires IS NULL OR requires IN ('overlay'));

COMMENT ON COLUMN nivek.command.requires IS
    'Capability the channel must have provisioned for this command to dispatch. '
    'NULL = unconditional. ''overlay'' = a non-revoked nivek.overlay_device row. '
    'Distinct from channel_command_settings, which carries per-channel preference.';


-- Step 3 -- ONLY after the bot ships with the handlers and the gate.
--
-- Triggers are stored lowercase. handleWebhookMessage lowercases the incoming
-- message (bot_read_message.go) but getGlobalEnabledCommands keys the dispatch
-- map on row.Trigger verbatim (bot.go), so a mixed-case trigger would never
-- match. The overlay registers "zeroG"; it is '!zerog' here.
--
-- Aliases are separate rows sharing a handler_key -- the schema has no alias
-- concept. They are the aliases the overlay already accepts, typos included.
--
-- min_role is 'everyone' throughout, matching what these commands do in the
-- overlay today. cooldown_secs is left at 0 because nothing enforces it yet;
-- seeding non-zero values would imply a limit that is not applied.

INSERT INTO nivek.command
    (trigger, kind, handler_key, min_role, scope, requires, enabled, description)
VALUES
    ('!b',         'builtin', 'overlay_beans',   'everyone', 'global', 'overlay', TRUE,
     'Spawns beans into the overlay physics scene. Optional count, e.g. "!b 20".'),

    ('!n',         'builtin', 'overlay_name',    'everyone', 'global', 'overlay', TRUE,
     'Drops your username into the overlay as a physics object.'),

    ('!shake',     'builtin', 'overlay_shake',   'everyone', 'global', 'overlay', TRUE,
     'Shakes everything currently in the overlay physics scene.'),

    ('!laser',     'builtin', 'overlay_laser',   'everyone', 'global', 'overlay', TRUE,
     'Fires lasers across the overlay.'),

    ('!nuke',      'builtin', 'overlay_nuke',    'everyone', 'global', 'overlay', TRUE,
     'Clears the overlay physics scene. Use when the beans have won.'),

    ('!zerog',     'builtin', 'overlay_zero_g',  'everyone', 'global', 'overlay', TRUE,
     'Toggles zero gravity in the overlay physics scene.'),
    ('!0g',        'builtin', 'overlay_zero_g',  'everyone', 'global', 'overlay', TRUE,
     'Alias of !zerog.'),

    ('!g',         'builtin', 'overlay_grenade', 'everyone', 'global', 'overlay', TRUE,
     'Spawns grenades in the overlay physics scene. Optional count, e.g. "!g 3".'),
    ('!grenade',   'builtin', 'overlay_grenade', 'everyone', 'global', 'overlay', TRUE,
     'Alias of !g.'),
    ('!grenades',  'builtin', 'overlay_grenade', 'everyone', 'global', 'overlay', TRUE,
     'Alias of !g.'),
    ('!granade',   'builtin', 'overlay_grenade', 'everyone', 'global', 'overlay', TRUE,
     'Alias of !g (misspelling the overlay already accepts).'),
    ('!grandma',   'builtin', 'overlay_grenade', 'everyone', 'global', 'overlay', TRUE,
     'Alias of !g (misspelling the overlay already accepts).'),

    ('!snow',      'builtin', 'overlay_snow',    'everyone', 'global', 'overlay', TRUE,
     'Turns on the snow effect over the overlay.')
ON CONFLICT (trigger) WHERE scope = 'global' DO NOTHING;


-- Deliberately NOT global: the overlay's remaining commands play specific audio
-- files from the streamer's own sfx folder (!d, !quack, !mika, !scawy, !irad,
-- !beginning, !yippee) or drive that streamer's setup (!toggle_music, !pandano
-- restarts a named OBS source). None of them mean anything in a fresh overlay
-- install, so they stay channel-scoped however they end up expressed.
