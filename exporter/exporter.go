package exporter

import (
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

func (e *Exporter) processMetrics(deviceData []*types.Device, ch chan<- prometheus.Metric) error {
	for _, d := range deviceData {
		ch <- prometheus.MustNewConstMetric(temperature, prometheus.GaugeValue, d.NewestEvents.Temperature.Value, d.Name, d.ID)
		ch <- prometheus.MustNewConstMetric(humidity, prometheus.GaugeValue, d.NewestEvents.Humidity.Value, d.Name, d.ID)
		ch <- prometheus.MustNewConstMetric(illumination, prometheus.GaugeValue, d.NewestEvents.Illumination.Value, d.Name, d.ID)
	}

	return nil
}
