package weather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sirupsen/logrus"
)

const GetWindyWeatherReportUrl = "https://api.windy.com/api/point-forecast/v2"

// WindyWeatherReportRequest based on docs: https://api.windy.com/point-forecast/docs
type WindyWeatherReportRequest struct {
	Lat        float64  `json:"lat"`
	Lon        float64  `json:"lon"`
	Model      string   `json:"model"`
	Parameters []string `json:"parameters"`
	Levels     []string `json:"levels"`
	Key        string   `json:"key"` // Api Key
}

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

func (c Client) buildWindyWeatherReportRequestBody(lat, lon float64) WindyWeatherReportRequest {
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

func (c Client) fetchReportFromWindy(body WindyWeatherReportRequest) *WindyResponseBody {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		logrus.Errorf(
			"error marshalling weather report request in GetWeatherReport for (%s): %s",
			body,
			err.Error(),
		)
		return nil
	}

	windyResponse, err := c.httpClient.Post(
		GetWindyWeatherReportUrl,
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		logrus.Errorf(
			"error fetching report from windy: %s",
			err.Error(),
		)
		return nil
	}
	defer windyResponse.Body.Close()

	windyResponseBody, err := io.ReadAll(windyResponse.Body)
	if err != nil {
		logrus.Errorf(
			"error parsing report from windy: %s",
			err.Error(),
		)
		return nil
	}

	logrus.Infof("windoy response:")
	fmt.Println(string(windyResponseBody))

	var response WindyResponseBody
	if err := json.Unmarshal(windyResponseBody, &response); err != nil {
		logrus.Errorf(
			"error parsing report from windy: %s",
			err.Error(),
		)
		return nil
	}

	return &response
}
