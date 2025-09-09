package weather

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
	"github.com/suuuth/nivek/internal/libraries/weather"
)

func NewGetWeatherEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := getIP(c)

		weatherService := weather.NewWeatherReportService(nivek)
		report, err := weatherService.GetReport(ip)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error fetching weather report: %s", err.Error()),
			})
		}

		return c.JSON(http.StatusOK, report)
	}
}

// getIP gets IP off request. If a IP value exists in the JSON body, then use that value instead
func getIP(c echo.Context) string {
	ip := c.RealIP()
	logrus.Infof("initial ip: %s", ip)

	var fetchedIP struct {
		Ip string `json:"ip"`
	}
	err := c.Bind(&fetchedIP)
	if err == nil {
		ip = fetchedIP.Ip
	}

	return ip
}
