-- Production apply for the overlay device start_seq cursor floor.
-- Safe to re-run: ADD COLUMN IF NOT EXISTS is idempotent.
--
-- Why: a newly minted overlay token gets a new device id but the same user_id,
-- and a freshly paired overlay connects with since=0. The connect handler then
-- replays the ENTIRE per-user durable log (every cheer/redemption/power-up/
-- extension event ever stored -- redemptions are replayable, only chat commands
-- are excluded), so pairing a new device re-executes months of old redemptions
-- on stream even though nothing new happened.
--
-- start_seq floors replay: a device begins at the user's current MAX(seq) at the
-- moment it is minted, so only events that land AFTER pairing are delivered.
--
-- Deploy order: this column is additive and the old core-api binary neither
-- reads nor writes it (its inserts omit it -> DEFAULT 0 -> prior behaviour), so
-- this can be applied before OR after the deploy with no coordination. The new
-- binary populates start_seq inside CreateDevice's existing transaction.

ALTER TABLE nivek.overlay_device ADD COLUMN IF NOT EXISTS start_seq BIGINT NOT NULL DEFAULT 0;
