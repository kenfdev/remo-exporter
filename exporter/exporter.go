package exporter

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/log"
	"github.com/kenfdev/remo-exporter/types"
)

const (
	namespace = "remo"
)

// Metrics descriptions
var (
	temperature = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "temperature"),
		"The temperature of the remo device",
		[]string{"name", "id"}, nil,
	)

	humidity = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "humidity"),
		"The humidity of the remo device",
		[]string{"name", "id"}, nil,
	)

	illumination = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "illumination"),
		"The illumination of the remo device",
		[]string{"name", "id"}, nil,
	)

	motion = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "motion"),
		"The motion of the remo device",
		[]string{"name", "id"}, nil,
	)

	rateLimitLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "x_rate_limit_limit"),
		"The rate limit for the remo API",
		nil, nil,
	)

	rateLimitReset = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "x_rate_limit_reset"),
		"The time in which the rate limit for the remo API will be reset",
		nil, nil,
	)

	rateLimitRemaining = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "x_rate_limit_remaining"),
		"The remaining number of request for the remo API",
		nil, nil,
	)

	httpRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "The total number of requests labeled by response code",
	},
		[]string{"code"},
	)
)

// Exporter collects ECS clusters metrics
type Exporter struct {
	client RemoGatherer // Custom ECS client to get information from the clusters
}

// NewExporter returns an initialized exporter
func NewExporter(config *config.Config, client RemoGatherer) (*Exporter, error) {

	return &Exporter{
		client: client,
	}, nil

}

// Describe is to describe the metrics for Prometheus
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- temperature
	ch <- humidity
	ch <- illumination
	ch <- motion
	ch <- rateLimitLimit
	ch <- rateLimitReset
	ch <- rateLimitRemaining
	httpRequestsTotal.Describe(ch)
}

// Collect collects data to be consumed by prometheus
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	data, err := e.client.GetDevices()
	if err != nil {
		log.Errorf("Fetching device stats failed: %v", err)
		return
	}

	err = e.processMetrics(data, ch)
	if err != nil {
		log.Errorf("Processing the metrics failed: %v", err)
		return
	}

}

func (e *Exporter) processMetrics(devicesResult *types.GetDevicesResult, ch chan<- prometheus.Metric) error {
	for _, d := range devicesResult.Devices {
		if d.NewestEvents.Temperature != nil {
			ch <- prometheus.MustNewConstMetric(temperature, prometheus.GaugeValue, d.NewestEvents.Temperature.Value, d.Name, d.ID)
		}
		if d.NewestEvents.Humidity != nil {
			ch <- prometheus.MustNewConstMetric(humidity, prometheus.GaugeValue, d.NewestEvents.Humidity.Value, d.Name, d.ID)
		}
		if d.NewestEvents.Illumination != nil {
			ch <- prometheus.MustNewConstMetric(illumination, prometheus.GaugeValue, d.NewestEvents.Illumination.Value, d.Name, d.ID)
		}
		if d.NewestEvents.Motion != nil {
			ch <- prometheus.MustNewConstMetric(motion, prometheus.GaugeValue, float64(d.NewestEvents.Motion.CreatedAt.Unix()), d.Name, d.ID)
		}
	}

	if devicesResult.Meta != nil {
		ch <- prometheus.MustNewConstMetric(rateLimitLimit, prometheus.GaugeValue, devicesResult.Meta.RateLimitLimit)
		ch <- prometheus.MustNewConstMetric(rateLimitRemaining, prometheus.GaugeValue, devicesResult.Meta.RateLimitRemaining)
		ch <- prometheus.MustNewConstMetric(rateLimitReset, prometheus.GaugeValue, devicesResult.Meta.RateLimitReset)
	}

	if devicesResult.StatusCode > 0 {
		if !devicesResult.IsCache {
			// increment the counter only if it's not a cache
			httpRequestsTotal.WithLabelValues(strconv.Itoa(devicesResult.StatusCode)).Inc()
		}
		httpRequestsTotal.Collect(ch)
	}

	return nil
}
