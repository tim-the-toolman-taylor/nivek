package weather

//
//import (
//	"github.com/ipinfo/go/v2/ipinfo"
//	"github.com/sirupsen/logrus"
//	"github.com/suuuth/nivek/internal/libraries/config"
//	"github.com/suuuth/nivek/internal/libraries/nivek"
//)
//
//type Client struct {
//	nivek  nivek.NivekService
//	client *ipinfo.Client
//}
//
//func NewWeatherClient(nivek nivek.NivekService) *Client {
//	return &Client{
//		nivek,
//		ipinfo.NewClient(nil, nil, config.GetConfig().IPInfo.Token),
//	}
//}
//
//func (c Client) getIP() (string, error) {
//	if ip, err := c.client.GetIPAddr(); err != nil {
//		logrus.Errorf("error getting IP address in weather module: %s", err.Error())
//		return "", err
//	} else {
//		return ip, nil
//	}
//}
//
//func (c Client) getCity() (string, error) {
//	if city, err := c.client.Get
//}
