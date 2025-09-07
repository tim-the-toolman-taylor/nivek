package weather

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/cmd/core-api/utility"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/weather"
)

func NewGetWeatherEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := c.RealIP()
		logrus.Infof("initial ip: %s", ip)

		var fetchedIP struct {
			Ip string `json:"ip"`
		}
		if err := c.Bind(&fetchedIP); err != nil {
			return utility.RejectBadRequest(c)
		}

		ip = fetchedIP.Ip
		weatherClient := weather.NewWeatherClient(nivek)

		report, err := weatherClient.GetWeatherReport(ip)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error fetching weather report: %s", err.Error()),
			})
		}

		return c.JSON(http.StatusOK, report)
	}
}
