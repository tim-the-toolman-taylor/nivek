package stalk

import (
	"fmt"
	"strings"

	userlib "github.com/tim-the-toolman-taylor/nivek/internal/libraries/user"
)

func channelLogin(u *userlib.User) (string, error) {
	if u == nil || u.TwitchLogin == nil || strings.TrimSpace(*u.TwitchLogin) == "" {
		return "", fmt.Errorf("no linked twitch account")
	}
	return strings.ToLower(*u.TwitchLogin), nil
}

func setByLogin(u *userlib.User) string {
	if u == nil || u.TwitchLogin == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*u.TwitchLogin))
}
