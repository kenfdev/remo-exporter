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

	normalElectricEnergy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "normal_direction_cumulative_electric_energy"),
		"The raw value for cumulative electric energy in normal direction",
		[]string{"name", "id"}, nil,
	)

	reverseElectricEnergy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "reverse_direction_cumulative_electric_energy"),
		"The raw value for cumulative electric energy in reverse direction",
		[]string{"name", "id"}, nil,
	)

	coefficient = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "coefficient"),
		"The coefficient for cumulative electric energy",
		[]string{"name", "id"}, nil,
	)

	electricEnergyUnit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cumulative_electric_energy_unit_kilowatt_hour"),
		"The unit in kWh for cumulative electric energy",
		[]string{"name", "id"}, nil,
	)

	electricEnergyDigits = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "cumulative_electric_energy_effective_digits"),
		"The number of effective digits for cumulative electric energy",
		[]string{"name", "id"}, nil,
	)

	measuredInstantaneousEnergy = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "measured_instantaneous_energy_watt"),
		"The measured instantaneous energy in W",
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
		[]string{"code", "api"},
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
	ch <- normalElectricEnergy
	ch <- reverseElectricEnergy
	ch <- coefficient
	ch <- electricEnergyUnit
	ch <- electricEnergyDigits
	ch <- measuredInstantaneousEnergy
	ch <- rateLimitLimit
	ch <- rateLimitReset
	ch <- rateLimitRemaining
	httpRequestsTotal.Describe(ch)
}

// Collect collects data to be consumed by prometheus
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	devices, err := e.client.GetDevices()
	if err != nil {
		log.Errorf("Fetching device stats failed: %v", err)
		return
	}

	appliances, err := e.client.GetAppliances()
	if err != nil {
		log.Errorf("Fetching appliances stats failed: %v", err)
		return
	}

	err = e.processMetrics(devices, appliances, ch)
	if err != nil {
		log.Errorf("Processing the metrics failed: %v", err)
		return
	}

}

func (e *Exporter) processMetrics(devicesResult *types.GetDevicesResult, appliancesResult *types.GetAppliancesResult, ch chan<- prometheus.Metric) error {
	for _, d := range devicesResult.Devices {
		if d.NewestEvents == nil {
			continue
		}
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

	sms := getSmartMeters(appliancesResult.Appliances)
	for _, sm := range sms {
		info, err := energyInfo(sm)
		if err != nil {
			log.Errorf("failed to get EnergyInfo: %v", err)
			continue
		}
		ch <- prometheus.MustNewConstMetric(normalElectricEnergy, prometheus.CounterValue, float64(info.NormalEnergy), sm.Device.Name, sm.Device.ID)
		ch <- prometheus.MustNewConstMetric(reverseElectricEnergy, prometheus.CounterValue, float64(info.ReverseEnergy), sm.Device.Name, sm.Device.ID)
		ch <- prometheus.MustNewConstMetric(coefficient, prometheus.GaugeValue, float64(info.Coefficient), sm.Device.Name, sm.Device.ID)
		ch <- prometheus.MustNewConstMetric(electricEnergyUnit, prometheus.GaugeValue, info.EnergyUnit, sm.Device.Name, sm.Device.ID)
		ch <- prometheus.MustNewConstMetric(electricEnergyDigits, prometheus.GaugeValue, float64(info.EffectiveDigits), sm.Device.Name, sm.Device.ID)
		ch <- prometheus.MustNewConstMetric(measuredInstantaneousEnergy, prometheus.GaugeValue, float64(info.MeasuredInstantaneous), sm.Device.Name, sm.Device.ID)
	}

	if appliancesResult.Meta != nil {
		ch <- prometheus.MustNewConstMetric(rateLimitLimit, prometheus.GaugeValue, appliancesResult.Meta.RateLimitLimit)
		ch <- prometheus.MustNewConstMetric(rateLimitRemaining, prometheus.GaugeValue, appliancesResult.Meta.RateLimitRemaining)
		ch <- prometheus.MustNewConstMetric(rateLimitReset, prometheus.GaugeValue, appliancesResult.Meta.RateLimitReset)
	} else if devicesResult.Meta != nil {
		ch <- prometheus.MustNewConstMetric(rateLimitLimit, prometheus.GaugeValue, devicesResult.Meta.RateLimitLimit)
		ch <- prometheus.MustNewConstMetric(rateLimitRemaining, prometheus.GaugeValue, devicesResult.Meta.RateLimitRemaining)
		ch <- prometheus.MustNewConstMetric(rateLimitReset, prometheus.GaugeValue, devicesResult.Meta.RateLimitReset)
	}

	if devicesResult.StatusCode > 0 {
		if !devicesResult.IsCache {
			// increment the counter only if it's not a cache
			httpRequestsTotal.WithLabelValues(strconv.Itoa(devicesResult.StatusCode), "devices").Inc()
		}
	}
	if appliancesResult.StatusCode > 0 {
		if !appliancesResult.IsCache {
			// increment the counter only if it's not a cache
			httpRequestsTotal.WithLabelValues(strconv.Itoa(appliancesResult.StatusCode), "appliances").Inc()
		}
	}
	httpRequestsTotal.Collect(ch)

	return nil
}
