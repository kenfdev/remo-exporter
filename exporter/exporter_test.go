package exporter_test

import (
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

var _ = Describe("Exporter", func() {
	var (
		mockCtrl *gomock.Controller
	)
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		mockCtrl.Finish()
	})
	Describe("Describe", func() {
		It("should describe the prometheus metrics", func() {
			remoClient := mocks.NewMockRemoGatherer(mockCtrl)

			c, _ := config.NewConfig()
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
				},
			}
			devices := []*types.Device{
				device,
			}
			remoClient.EXPECT().GetDevices().Return(devices, nil)

			c, _ := config.NewConfig()
			e, err := NewExporter(c, remoClient)
			Expect(err).Should(BeNil())

			ch := make(chan prometheus.Metric)
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
		})
	})
})
