package fishing

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	fshService "github.com/tim-the-toolman-taylor/nivek/internal/libraries/fishing"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

func NewGetFishingScoreEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		user, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "internal server error",
			})
		}

		fishingService := fshService.NewService(nivek, *user.TwitchLogin)
		fishScoresAsChatter, errFsh := fishingService.GetUserFishScore()
		if errFsh != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching fish score for user [%s]: %s",
					*user.TwitchLogin, errFsh.Error(),
				),
			})
		}

		fishScoresInChannel, errFshChan := fishingService.GetChannelFishScore()
		if errFshChan != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf(
					"error fetching fish score for channel [%s]: %s",
					*user.TwitchLogin, errFshChan.Error(),
				),
			})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"as_chatter": fishScoresAsChatter,
			"as_channel": fishScoresInChannel,
		})
	}
}
