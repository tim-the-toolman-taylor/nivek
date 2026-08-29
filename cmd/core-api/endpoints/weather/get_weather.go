package weather

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/weather"
)

func NewGetWeatherEndpoint(nivek nivek.NivekService) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := getIP(c, nivek)

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
func getIP(c echo.Context, nivek nivek.NivekService) string {
	ip := c.RealIP()
	nivek.Logger().Infof("initial ip: %s", ip)

	var fetchedIP struct {
		Ip string `json:"ip"`
	}
	err := c.Bind(&fetchedIP)
	if err == nil {
		ip = fetchedIP.Ip
	}

	return ip
}
