package exporter_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/exporter"
	authhttp "github.com/kenfdev/remo-exporter/http"
)

func newRegressionClient(t *testing.T, server *httptest.Server, ttl int) *exporter.RemoClient {
	t.Helper()
	client, err := exporter.NewRemoClient(&config.Config{
		APIBaseURL:               server.URL,
		CacheInvalidationSeconds: ttl,
	}, authhttp.NewAuthHttpClient("test-token"))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func setRateLimitHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Rate-Limit-Limit", "30")
	w.Header().Set("X-Rate-Limit-Remaining", "29")
	w.Header().Set("X-Rate-Limit-Reset", "1700000000")
}

func TestRemoRejectsMalformedDevicesWithoutCaching(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRateLimitHeaders(w)
		if requests.Add(1) == 1 {
			fmt.Fprint(w, `[{"id":"partial","temperature_offset":"invalid"}]`)
			return
		}
		fmt.Fprint(w, `[{"id":"recovered","temperature_offset":0}]`)
	}))
	t.Cleanup(server.Close)
	client := newRegressionClient(t, server, 60)

	result, err := client.GetDevices()
	if err == nil || result != nil {
		t.Errorf("malformed response = (%+v, %v), want nil result and error", result, err)
	}
	result, err = client.GetDevices()
	if err != nil {
		t.Fatal(err)
	}
	if result.IsCache || len(result.Devices) != 1 || result.Devices[0].ID != "recovered" {
		t.Fatalf("recovery result = %+v, want uncached recovered device", result)
	}
	cached, err := client.GetDevices()
	if err != nil {
		t.Fatal(err)
	}
	if !cached.IsCache || len(cached.Devices) != 1 || cached.Devices[0].ID != "recovered" || requests.Load() != 2 {
		t.Fatalf("cache result = %+v, requests = %d, want recovered device after two requests", cached, requests.Load())
	}
}

func TestRemoDecodesFractionalOffsetsAtBothEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRateLimitHeaders(w)
		switch r.URL.Path {
		case "/1/devices":
			fmt.Fprint(w, `[{"id":"device","temperature_offset":-0.5}]`)
		case "/1/appliances":
			fmt.Fprint(w, `[{"id":"appliance","device":{"id":"device","temperature_offset":0.5}}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := newRegressionClient(t, server, 60)

	devices, err := client.GetDevices()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices.Devices) != 1 || float64(devices.Devices[0].TemperatureOffset) != -0.5 {
		t.Errorf("devices = %+v, want temperature offset -0.5", devices.Devices)
	}
	appliances, err := client.GetAppliances()
	if err != nil {
		t.Fatal(err)
	}
	if len(appliances.Appliances) != 1 || appliances.Appliances[0].Device == nil || float64(appliances.Appliances[0].Device.TemperatureOffset) != 0.5 {
		t.Errorf("appliances = %+v, want nested temperature offset 0.5", appliances.Appliances)
	}
}

type cacheObservation struct {
	statusCode int
	isCache    bool
	id         string
	remaining  float64
}

var regressionEndpoints = []struct {
	name string
	get  func(*exporter.RemoClient) (cacheObservation, error)
}{
	{
		name: "devices",
		get: func(c *exporter.RemoClient) (cacheObservation, error) {
			r, err := c.GetDevices()
			if err != nil {
				return cacheObservation{}, err
			}
			result := cacheObservation{statusCode: r.StatusCode, isCache: r.IsCache, remaining: r.Meta.RateLimitRemaining}
			if len(r.Devices) != 0 {
				result.id = r.Devices[0].ID
			}
			return result, nil
		},
	},
	{
		name: "appliances",
		get: func(c *exporter.RemoClient) (cacheObservation, error) {
			r, err := c.GetAppliances()
			if err != nil {
				return cacheObservation{}, err
			}
			result := cacheObservation{statusCode: r.StatusCode, isCache: r.IsCache, remaining: r.Meta.RateLimitRemaining}
			if len(r.Appliances) != 0 {
				result.id = r.Appliances[0].ID
			}
			return result, nil
		},
	},
}

func TestRemoRetriesNon200Responses(t *testing.T) {
	for _, endpoint := range regressionEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				setRateLimitHeaders(w)
				if requests.Add(1) == 1 {
					w.Header().Set("X-Rate-Limit-Remaining", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					fmt.Fprint(w, `{"message":"rate limited"}`)
					return
				}
				fmt.Fprint(w, `[{"id":"recovered"}]`)
			}))
			t.Cleanup(server.Close)
			client := newRegressionClient(t, server, 60)

			first, err := endpoint.get(client)
			if err != nil || first != (cacheObservation{statusCode: 429, remaining: 0}) {
				t.Fatalf("first response = %+v, %v, want uncached 429 and rate limit 0", first, err)
			}
			second, err := endpoint.get(client)
			if err != nil || second != (cacheObservation{statusCode: 200, id: "recovered", remaining: 29}) {
				t.Fatalf("retry = %+v, %v, want uncached recovered data", second, err)
			}
			third, err := endpoint.get(client)
			if err != nil || third != (cacheObservation{statusCode: 200, isCache: true, id: "recovered", remaining: 29}) || requests.Load() != 2 {
				t.Fatalf("cached retry = %+v, %v, requests = %d", third, err, requests.Load())
			}
		})
	}
}

func TestRemoZeroTTLAlwaysFetches(t *testing.T) {
	for _, endpoint := range regressionEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				setRateLimitHeaders(w)
				fmt.Fprintf(w, `[{"id":"response-%d"}]`, requests.Add(1))
			}))
			t.Cleanup(server.Close)
			client := newRegressionClient(t, server, 0)
			for i := 1; i <= 2; i++ {
				result, err := endpoint.get(client)
				want := cacheObservation{statusCode: 200, id: fmt.Sprintf("response-%d", i), remaining: 29}
				if err != nil || result != want {
					t.Fatalf("response %d = %+v, %v, want %+v", i, result, err, want)
				}
			}
		})
	}
}

func TestRemoCacheTTLStartsBeforeFetch(t *testing.T) {
	for _, endpoint := range regressionEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := requests.Add(1)
				if n == 1 {
					time.Sleep(1100 * time.Millisecond)
				}
				setRateLimitHeaders(w)
				fmt.Fprintf(w, `[{"id":"response-%d"}]`, n)
			}))
			t.Cleanup(server.Close)
			client := newRegressionClient(t, server, 1)
			for i := 1; i <= 2; i++ {
				result, err := endpoint.get(client)
				want := cacheObservation{statusCode: 200, id: fmt.Sprintf("response-%d", i), remaining: 29}
				if err != nil || result != want {
					t.Fatalf("response %d = %+v, %v, want %+v", i, result, err, want)
				}
			}
		})
	}
}

func TestRemoConcurrentCacheMissesShareOneFetch(t *testing.T) {
	for _, endpoint := range regressionEndpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				setRateLimitHeaders(w)
				fmt.Fprint(w, `[{"id":"shared"}]`)
			}))
			t.Cleanup(server.Close)
			client := newRegressionClient(t, server, 60)
			const workers = 32
			start := make(chan struct{})
			var wg sync.WaitGroup
			var misses atomic.Int32
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					result, err := endpoint.get(client)
					if err != nil || result.id != "shared" || result.statusCode != 200 || result.remaining != 29 {
						t.Errorf("concurrent result = %+v, %v", result, err)
						return
					}
					if !result.isCache {
						misses.Add(1)
					}
				}()
			}
			close(start)
			wg.Wait()
			if requests.Load() != 1 || misses.Load() != 1 {
				t.Fatalf("requests = %d, cache misses = %d, want one of each", requests.Load(), misses.Load())
			}
		})
	}
}

func TestRemoCachesAreIndependent(t *testing.T) {
	deviceStarted := make(chan struct{})
	releaseDevice := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setRateLimitHeaders(w)
		if r.URL.Path == "/1/devices" {
			close(deviceStarted)
			<-releaseDevice
		}
		fmt.Fprint(w, `[{"id":"independent"}]`)
	}))
	t.Cleanup(server.Close)
	client := newRegressionClient(t, server, 60)
	deviceDone := make(chan error, 1)
	go func() {
		_, err := client.GetDevices()
		deviceDone <- err
	}()
	defer func() {
		close(releaseDevice)
		if err := <-deviceDone; err != nil {
			t.Error(err)
		}
	}()
	select {
	case <-deviceStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("device request did not start")
	}
	applianceDone := make(chan error, 1)
	go func() {
		result, err := client.GetAppliances()
		if err == nil && (result.IsCache || len(result.Appliances) != 1 || result.Appliances[0].ID != "independent") {
			err = fmt.Errorf("appliance result = %+v, want independent uncached result", result)
		}
		applianceDone <- err
	}()
	select {
	case err := <-applianceDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("appliance request blocked behind device request")
	}
}
