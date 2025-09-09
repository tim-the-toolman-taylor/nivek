package weather

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/ipinfo/go/v2/ipinfo"
	"github.com/sirupsen/logrus"
	"github.com/suuuth/nivek/internal/libraries/nivek"
)

type NivekWeatherReport struct {
	City string `json:"city"`
	Temp string `json:"temp"`
}

type ReportService struct {
	nivek.NivekService
	infoClient  *ipinfo.Client
	windyClient *WindyClient
}

func NewWeatherReportService(nivek nivek.NivekService) *ReportService {
	return &ReportService{
		NivekService: nivek,
		infoClient:   ipinfo.NewClient(nil, nil, nivek.CommonConfig().IPInfo.Token),
		windyClient:  NewWindyClient(nivek),
	}
}

func (s *ReportService) GetReport(ip string) (*NivekWeatherReport, error) {
	info, err := s.getInfo(ip)
	if err != nil {
		return nil, fmt.Errorf(
			"error fetching IP info during GetTemp for ip (%s): %s", ip, err.Error(),
		)
	}

	lat, lon := s.getLatAndLon(info)
	temp, err := s.windyClient.GetTemp(lat, lon)
	if err != nil {
		return nil, err
	}

	nivekReport := NivekWeatherReport{
		City: info.City,
		Temp: temp,
	}

	return &nivekReport, nil
}

func (s *ReportService) getInfo(ip string) (*ipinfo.Core, error) {
	return s.infoClient.GetIPInfo(net.ParseIP(ip))
}

func (s *ReportService) getLatAndLon(info *ipinfo.Core) (float64, float64) {
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
