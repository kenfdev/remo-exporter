package exporter

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
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
	GetDevices() (*types.GetDevicesResult, error)
}

type Metrics struct {
	StatusCode int
	Meta       *types.Meta
	Devices    []*types.Device
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

func getMetaStats(header http.Header) *types.Meta {
	limitStr := header.Get("X-Rate-Limit-Limit")
	limit, err := strconv.ParseFloat(limitStr, 64)
	if err != nil {
		log.Errorf("Error parsing X-Rate-Limit-Limit: %s", err.Error())
		limit = 0
	}

	remainingStr := header.Get("X-Rate-Limit-Remaining")
	remaining, err := strconv.ParseFloat(remainingStr, 64)
	if err != nil {
		log.Errorf("Error parsing X-Rate-Limit-Remaining: %s", err.Error())
		remaining = 0
	}

	resetStr := header.Get("X-Rate-Limit-Reset")
	reset, err := strconv.ParseFloat(resetStr, 64)
	if err != nil {
		log.Errorf("Error parsing X-Rate-Limit-Reset: %s", err.Error())
		reset = 0
	}

	return &types.Meta{
		RateLimitLimit:     limit,
		RateLimitRemaining: remaining,
		RateLimitReset:     reset,
	}
}

// GetDevices will get the devices from the Remo API
func (c *RemoClient) GetDevices() (*types.GetDevicesResult, error) {
	now := int(time.Now().Unix())

	if now < c.cacheExpirationTimestamp {
		log.Infof("Returning cache. Cache valid for %d seconds", c.cacheExpirationTimestamp-now)
		result := &types.GetDevicesResult{
			StatusCode: c.cachedMetrics.StatusCode,
			Meta:       c.cachedMetrics.Meta,
			Devices:    c.cachedMetrics.Devices,
			IsCache:    true,
		}
		return result, nil
	}

	url := c.baseURL + "/1/devices"
	resp, err := c.authClient.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	data := []*types.Device{}
	if resp.StatusCode == 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(bodyBytes, &data)

		// only update invalidation time on successful requests
		c.cacheExpirationTimestamp = now + c.cacheInvalidationSeconds
		log.Infof("Fetched data from the remote API. Caching until %d", c.cacheExpirationTimestamp)
	}

	meta := getMetaStats(resp.Header)
	result := &types.GetDevicesResult{
		StatusCode: resp.StatusCode,
		Meta:       meta,
		Devices:    data,
		IsCache:    false,
	}

	c.cachedMetrics.StatusCode = result.StatusCode
	c.cachedMetrics.Meta = result.Meta
	c.cachedMetrics.Devices = result.Devices

	return result, nil
}
