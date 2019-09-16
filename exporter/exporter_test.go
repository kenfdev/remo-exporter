package exporter_test

import (
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	. "github.com/kenfdev/remo-exporter/exporter"
	"github.com/kenfdev/remo-exporter/mocks"
)

type metricResult struct {
	value  float64
	labels map[string]string
}

func labels2Map(labels []*dto.LabelPair) map[string]string {
	res := map[string]string{}
	for _, l := range labels {
		res[l.GetName()] = l.GetValue()
	}
	return res
}

func readGauge(g prometheus.Metric) metricResult {
	m := &dto.Metric{}
	g.Write(m)

	return metricResult{
		value:  m.GetGauge().GetValue(),
		labels: labels2Map(m.GetLabel()),
	}
}

func readCounter(g prometheus.Counter) metricResult {
	m := &dto.Metric{}
	g.Write(m)

	return metricResult{
		value:  m.GetCounter().GetValue(),
		labels: labels2Map(m.GetLabel()),
	}
}

var _ = Describe("Exporter", func() {
	var (
		mockCtrl   *gomock.Controller
		mockReader *mocks.MockReader
	)
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockReader = mocks.NewMockReader(mockCtrl)
	})
	AfterEach(func() {
		mockCtrl.Finish()
	})
	Describe("Describe", func() {
		It("should describe the prometheus metrics", func() {
			remoClient := mocks.NewMockRemoGatherer(mockCtrl)

			c, _ := config.NewConfig(mockReader)
			e, err := NewExporter(c, remoClient)
			Expect(err).Should(BeNil())

			ch := make(chan *prometheus.Desc)
			go e.Describe(ch)

			d := (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_temperature", help: "The temperature of the remo device", constLabels: {}, variableLabels: [name id]}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_humidity", help: "The humidity of the remo device", constLabels: {}, variableLabels: [name id]}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_illumination", help: "The illumination of the remo device", constLabels: {}, variableLabels: [name id]}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_motion", help: "The motion of the remo device", constLabels: {}, variableLabels: [name id]}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_x_rate_limit_limit", help: "The rate limit for the remo API", constLabels: {}, variableLabels: []}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_x_rate_limit_reset", help: "The time in which the rate limit for the remo API will be reset", constLabels: {}, variableLabels: []}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_x_rate_limit_remaining", help: "The remaining number of request for the remo API", constLabels: {}, variableLabels: []}`))
			d = (<-ch)
			Expect(d.String()).To(Equal(`Desc{fqName: "remo_http_requests_total", help: "The total number of requests labeled by response code", constLabels: {}, variableLabels: [code]}`))
		})
	})

	Describe("Collect", func() {
		It("should collect metrics from the devices", func() {
			remoClient := mocks.NewMockRemoGatherer(mockCtrl)

			device := &types.Device{
				Name: "some_device_name",
				ID:   "some_device_id",
				NewestEvents: &types.Event{
					Temperature: &types.SensorValue{
						Value: 50.0,
					},
					Humidity: &types.SensorValue{
						Value: 60.0,
					},
					Illumination: &types.SensorValue{
						Value: 40.0,
					},
					Motion: &types.SensorValue{
						CreatedAt: time.Now(),
						Value:     1.0,
					},
				},
			}
			result := &types.GetDevicesResult{
				StatusCode: 200,
				Devices:    []*types.Device{device},
				Meta: &types.Meta{
					RateLimitLimit:     30.0,
					RateLimitRemaining: 29.0,
					RateLimitReset:     1532778912,
				},
				IsCache: false,
			}
			remoClient.EXPECT().GetDevices().Return(result, nil)

			c, _ := config.NewConfig(mockReader)
			e, err := NewExporter(c, remoClient)
			Expect(err).Should(BeNil())

			ch := make(chan prometheus.Metric)
			defer close(ch)

			go e.Collect(ch)

			m := (<-ch).(prometheus.Metric)
			m2 := readGauge(m)
			Expect(m2.value).To(Equal(device.NewestEvents.Temperature.Value))
			Expect(m2.labels["name"]).To(Equal(device.Name))
			Expect(m2.labels["id"]).To(Equal(device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(device.NewestEvents.Humidity.Value))
			Expect(m2.labels["name"]).To(Equal(device.Name))
			Expect(m2.labels["id"]).To(Equal(device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(device.NewestEvents.Illumination.Value))
			Expect(m2.labels["name"]).To(Equal(device.Name))
			Expect(m2.labels["id"]).To(Equal(device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(float64(device.NewestEvents.Motion.CreatedAt.Unix())))
			Expect(m2.labels["name"]).To(Equal(device.Name))
			Expect(m2.labels["id"]).To(Equal(device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(result.Meta.RateLimitLimit))
			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(result.Meta.RateLimitRemaining))
			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(result.Meta.RateLimitReset))

			counter := (<-ch).(prometheus.Counter)
			m2 = readCounter(counter)
			Expect(m2.labels["code"]).To(Equal(strconv.Itoa(result.StatusCode)))
		})
	})
})
