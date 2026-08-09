package autoshout

import (
	"fmt"
	"strconv"

	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

// broadcasterID resolves the session user's Twitch broadcaster id (the
// auto_shout.twitch_id key) from their user record. auto_shout is keyed by the
// broadcaster's numeric Twitch id since the channelname column was removed, so
// a user with no linked Twitch identity has no shoutout list to manage.
func broadcasterID(u *userlib.User) (int, error) {
	if u == nil || u.TwitchID == nil || *u.TwitchID == "" {
		return 0, fmt.Errorf("user has no linked twitch_id")
	}
	id, err := strconv.Atoi(*u.TwitchID)
	if err != nil {
		return 0, fmt.Errorf("invalid twitch_id %q: %w", *u.TwitchID, err)
	}
	return id, nil
}
