package weather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type WindyResponseBody struct {
	Ts                  []int                  `json:"ts"`
	Units               WindyResponseBodyUnits `json:"units"`
	TempSurface         []float64              `json:"temp-surface"`
	Past3hPrecipSurface []float64              `json:"past3hprecip-surface"`
	WindUSurface        []float64              `json:"wind_u-surface"`
	WindVSurface        []float64              `json:"wind_v-surface"`
	PressureSurface     []float64              `json:"pressure-surface"`
	LCloudsSurface      []float64              `json:"lclouds-surface"`
	MCloudsSurface      []float64              `json:"mclouds-surface"`
	HCloudsSurface      []float64              `json:"hclouds-surface"`
}

type WindyResponseBodyUnits struct {
	TempSurface         string `json:"temp-surface"`
	Past3hPrecipSurface string `json:"past3hprecip-surface"`
	WindUSurface        string `json:"wind_u-surface"`
	WindVSurface        string `json:"wind_v-surface"`
	PressureSurface     string `json:"pressure-surface"`
	LCloudsSurface      string `json:"lclouds-surface"`
	MCloudsSurface      string `json:"mclouds-surface"`
	HCloudsSurface      string `json:"hclouds-surface"`
}

// WindyWeatherReportRequest based on docs: https://api.windy.com/point-forecast/docs
type WindyWeatherReportRequest struct {
	Lat        float64  `json:"lat"`
	Lon        float64  `json:"lon"`
	Model      string   `json:"model"`
	Parameters []string `json:"parameters"`
	Levels     []string `json:"levels"`
	Key        string   `json:"key"` // Api Key
}

const GetWindyWeatherReportUrl = "https://api.windy.com/api/point-forecast/v2"

type WindyClient struct {
	nivek      nivek.NivekService
	httpClient *http.Client
}

func NewWindyClient(nivek nivek.NivekService) *WindyClient {
	httpClient := &http.Client{}

	return &WindyClient{
		nivek,
		httpClient,
	}
}

func (c WindyClient) GetTemp(lat, lon float64) (string, error) {
	weatherReportRequestBody := c.buildRequestBody(lat, lon)
	data, err := c.fetchData(weatherReportRequestBody)
	if err != nil {
		return "", err
	}

	return c.formatTemp(data.TempSurface), nil
}

// buildRequestBody - handles building request body for weather api request
func (c WindyClient) buildRequestBody(lat, lon float64) WindyWeatherReportRequest {
	return WindyWeatherReportRequest{
		Lat:   lat,
		Lon:   lon,
		Model: "gfs",
		Parameters: []string{
			"temp",
			"precip",
			"wind",
			"pressure",
			"lclouds",
			"mclouds",
			"hclouds",
		},
		Levels: []string{"surface"},
		Key:    c.nivek.CommonConfig().Windy.Token,
	}
}

func (c WindyClient) fetchData(body WindyWeatherReportRequest) (*WindyResponseBody, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshalling weather report request in GetTemp for (%s): %s",
			string(jsonBody),
			err.Error(),
		)
	}

	windyResponse, err := c.httpClient.Post(
		GetWindyWeatherReportUrl,
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("error fetching report from windy: %s", err.Error())
	}
	defer windyResponse.Body.Close()

	windyResponseBody, err := io.ReadAll(windyResponse.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"error reading report from windy: %s",
			err.Error(),
		)
	}

	var response WindyResponseBody
	if err := json.Unmarshal(windyResponseBody, &response); err != nil {
		return nil, fmt.Errorf(
			"error unmarshalling report from windy: %s",
			err.Error(),
		)
	}

	return &response, nil
}

// formatTemp - Windy returns "surface temp" as a []float64 where each value is in Kelvin
// this method finds an average value and converts it to F
// then finally formats it nicely as a string with the unit provided
func (c WindyClient) formatTemp(values []float64) string {
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
