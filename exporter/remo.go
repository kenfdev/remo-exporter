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
	GetAppliances() (*types.GetAppliancesResult, error)
}

type DevicesMetrics struct {
	StatusCode int
	Meta       *types.Meta
	Devices    []*types.Device
}

type AppliancesMetrics struct {
	StatusCode int
	Meta       *types.Meta
	Appliances []*types.Appliance
}

// RemoClient is a http client who requests resources from the Remo API
type RemoClient struct {
	authClient                         authHttp.AuthHttpDoer
	baseURL                            string
	oauthToken                         string
	cachedDevicesMetrics               *DevicesMetrics
	cachedAppliancesMetrics            *AppliancesMetrics
	cacheInvalidationSeconds           int
	cacheDevicesExpirationTimestamp    int
	cacheAppliancesExpirationTimestamp int
}

// NewRemoClient will return an initialized RemoClient
func NewRemoClient(config *config.Config, authClient authHttp.AuthHttpDoer) (*RemoClient, error) {
	return &RemoClient{
		authClient:               authClient,
		baseURL:                  config.APIBaseURL,
		oauthToken:               config.OAuthToken,
		cacheInvalidationSeconds: config.CacheInvalidationSeconds,
		cachedDevicesMetrics:     &DevicesMetrics{},
		cachedAppliancesMetrics:  &AppliancesMetrics{},
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

	if now < c.cacheDevicesExpirationTimestamp {
		log.Infof("GetDevices: Returning cache. Cache valid for %d seconds", c.cacheDevicesExpirationTimestamp-now)
		result := &types.GetDevicesResult{
			StatusCode: c.cachedDevicesMetrics.StatusCode,
			Meta:       c.cachedDevicesMetrics.Meta,
			Devices:    c.cachedDevicesMetrics.Devices,
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
		c.cacheDevicesExpirationTimestamp = now + c.cacheInvalidationSeconds
		log.Infof("GetDevices: Fetched data from the remote API. Caching until %d", c.cacheDevicesExpirationTimestamp)
	}

	meta := getMetaStats(resp.Header)
	result := &types.GetDevicesResult{
		StatusCode: resp.StatusCode,
		Meta:       meta,
		Devices:    data,
		IsCache:    false,
	}

	c.cachedDevicesMetrics.StatusCode = result.StatusCode
	c.cachedDevicesMetrics.Meta = result.Meta
	c.cachedDevicesMetrics.Devices = result.Devices

	return result, nil
}

func (c *RemoClient) GetAppliances() (*types.GetAppliancesResult, error) {
	now := int(time.Now().Unix())

	if now < c.cacheAppliancesExpirationTimestamp {
		log.Infof("GetAppliances: Returning cache. Cache valid for %d seconds", c.cacheAppliancesExpirationTimestamp-now)
		result := &types.GetAppliancesResult{
			StatusCode: c.cachedAppliancesMetrics.StatusCode,
			Meta:       c.cachedAppliancesMetrics.Meta,
			Appliances: c.cachedAppliancesMetrics.Appliances,
			IsCache:    true,
		}
		return result, nil
	}

	url := c.baseURL + "/1/appliances"
	resp, err := c.authClient.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data []*types.Appliance
	if resp.StatusCode == 200 {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		err = json.Unmarshal(bodyBytes, &data)
		if err != nil {
			return nil, err
		}

		// only update invalidation time on successful requests
		c.cacheAppliancesExpirationTimestamp = now + c.cacheInvalidationSeconds
		log.Infof("GetAppliances: Fetched data from the remote API. Caching until %d", c.cacheAppliancesExpirationTimestamp)
	}

	meta := getMetaStats(resp.Header)
	result := &types.GetAppliancesResult{
		StatusCode: resp.StatusCode,
		Meta:       meta,
		Appliances: data,
		IsCache:    false,
	}

	c.cachedAppliancesMetrics.StatusCode = result.StatusCode
	c.cachedAppliancesMetrics.Meta = result.Meta
	c.cachedAppliancesMetrics.Appliances = result.Appliances

	return result, nil
}
