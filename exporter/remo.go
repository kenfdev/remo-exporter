package exporter

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"time"

	"github.com/kenfdev/remo-exporter/config"
	authHttp "github.com/kenfdev/remo-exporter/http"
	"github.com/kenfdev/remo-exporter/log"
	"github.com/kenfdev/remo-exporter/types"
)

const (
	maxServicesAPI = 10
)

// RemoGatherer gathers stats from the remo api
type RemoGatherer interface {
	GetDevices() ([]*types.Device, error)
}

type Metrics struct {
	Devices []*types.Device
}

// RemoClient is a http client who requests resources from the Remo API
type RemoClient struct {
	authClient               authHttp.AuthHttpDoer
	baseURL                  string
	oauthToken               string
	cachedMetrics            *Metrics
	cacheInvalidationSeconds int
	cacheExpirationTimestamp int
}

// NewRemoClient will return an initialized RemoClient
func NewRemoClient(config *config.Config, authClient authHttp.AuthHttpDoer) (*RemoClient, error) {
	return &RemoClient{
		authClient:               authClient,
		baseURL:                  config.APIBaseURL,
		oauthToken:               config.OAuthToken,
		cacheInvalidationSeconds: config.CacheInvalidationSeconds,
		cachedMetrics:            &Metrics{},
	}, nil
}

// GetDevices will get the devices from the Remo API
func (c *RemoClient) GetDevices() ([]*types.Device, error) {
	now := int(time.Now().Unix())

	if now < c.cacheExpirationTimestamp {
		log.Infof("Returning cache. Cache valid for %d seconds", c.cacheExpirationTimestamp-now)
		return c.cachedMetrics.Devices, nil
	}

	url := c.baseURL + "/1/devices"
	resp, err := c.authClient.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		var data []*types.Device
		json.Unmarshal(bodyBytes, &data)

		c.cachedMetrics.Devices = data
		c.cacheExpirationTimestamp = now + c.cacheInvalidationSeconds
		log.Infof("Fetched data from the remote API. Caching until %d", c.cacheExpirationTimestamp)
		return data, nil
	}

	return nil, errors.New("Request failed")
}
