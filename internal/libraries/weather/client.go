package weather

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ipinfo/go/v2/ipinfo"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/config"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type Client struct {
	nivek      nivek.NivekService
	infoClient *ipinfo.Client
	httpClient *http.Client
}

type NivekWeatherReport struct {
	City string `json:"city"`
	Temp string `json:"temp"`
}

func NewWeatherClient(nivek nivek.NivekService) *Client {
	httpClient := &http.Client{}

	return &Client{
		nivek,
		ipinfo.NewClient(nil, nil, config.GetConfig().IPInfo.Token),
		httpClient,
	}
}

func (c Client) GetWeatherReport(ip string) (*NivekWeatherReport, error) {
	info, err := c.getInfo(ip)
	if err != nil {
		logrus.Errorf("error fetching IP info during GetWeatherReport for ip (%s): %s", ip, err.Error())
		return nil, err
	}

	lat, lon := c.getLatAndLon(info)
	weatherReportRequestBody := c.buildWindyWeatherReportRequestBody(lat, lon)
	windyReport := c.fetchReportFromWindy(weatherReportRequestBody)
	if windyReport == nil {
		return nil, fmt.Errorf("error fetching WindyWeatherReport for ip (%s)", ip)
	}

	nivekReport := NivekWeatherReport{
		City: info.City,
		Temp: c.getTemp(windyReport.TempSurface),
	}

	return &nivekReport, nil
}

func (c Client) getInfo(ip string) (*ipinfo.Core, error) {
	return c.infoClient.GetIPInfo(net.ParseIP(ip))
}

func (c Client) getLatAndLon(info *ipinfo.Core) (float64, float64) {
	str := info.Location
	parts := strings.Split(str, ",")

	if len(parts) != 2 {
		logrus.Errorf(
			"error parsing lat/lon %s",
			str,
		)
		return 0, 0
	}

	latStr := strings.TrimSpace(parts[0])
	lonStr := strings.TrimSpace(parts[1])

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)

	if err1 != nil {
		logrus.Errorf("error parsing lat %s", err1.Error())
		return 0, 0
	}

	if err2 != nil {
		logrus.Errorf("error parsing lat/lon %s", err2.Error())
		return 0, 0
	}

	return lat, lon
}

// getTemp - Windy returns "surface temp" as a []float64 where each value is in Kelvin
// this method finds an average value and converts it to F
// then finally formats it nicely as a string with the unit provided
func (c Client) getTemp(values []float64) string {
	if len(values) == 0 {
		return "0°F"
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	avgKelvin := sum / float64(len(values))

	// Convert to Fahrenheit
	avgFahrenheit := (avgKelvin-273.15)*9/5 + 32

	// Round to nearest integer and convert to int64
	return fmt.Sprintf("%d°F", int64(math.Round(avgFahrenheit)))
}
