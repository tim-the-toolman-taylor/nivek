package weather

import (
	"fmt"
	"net"
	"net/http"

	"github.com/ipinfo/go/v2/ipinfo"
	"github.com/labstack/echo/v4"
	"github.com/suuuth/nivek/internal/libraries/config"
)

func NewGetWeatherEndpoint(e echo.Context) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := e.RealIP()

		client := ipinfo.NewClient(nil, nil, config.GetConfig().IPInfo.Token)

		info, err := client.GetIPInfo(net.ParseIP(ip))

		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{
				"error": fmt.Sprintf("error fetching ip info: %s", err.Error()),
			})
		}

		return e.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("IP info: %s", info.City),
		})
	}
}
