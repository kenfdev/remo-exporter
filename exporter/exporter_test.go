package exporter_test

import (
	"strconv"
	"time"

	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/mock/gomock"

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

func readCounter(g prometheus.Metric) metricResult {
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

			registry := prometheus.NewPedanticRegistry()
			Expect(registry.Register(e)).To(Succeed())
		})
	})

	Describe("Collect", func() {
		It("should collect metrics from the devices", func() {
			remoClient := mocks.NewMockRemoGatherer(mockCtrl)

			device := &types.Device{
				Name:            "some_device_name",
				ID:              "some_device_id",
				FirmwareVersion: "Remo/1.0.77-g808448c",
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
			remoClient.EXPECT().GetAppliances().Return(&types.GetAppliancesResult{}, nil)

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

		It("should collect metrics from Remo E lite", func() {
			remoClient := mocks.NewMockRemoGatherer(mockCtrl)

			device := &types.Device{
				Name:            "some_device_name",
				ID:              "some_device_id",
				FirmwareVersion: "Remo-E-lite/1.1.2",
			}
			appliance := &types.Appliance{
				ID:   "some_appliance_id",
				Type: "EL_SMART_METER",
				Device: &types.Device{
					Name: "some_device_name",
					ID:   "some_device_id",
				},
				SmartMeter: &types.SmartMeter{
					EchonetliteProperties: []*types.EchonetliteProperty{
						{
							Name: "coefficient",
							Epc:  211,
							Val:  "1",
						},
						{
							Name: "cumulative_electric_energy_effective_digits",
							Epc:  215,
							Val:  "6",
						},
						{
							Name: "normal_direction_cumulative_electric_energy",
							Epc:  224,
							Val:  "50851",
						},
						{
							Name: "cumulative_electric_energy_unit",
							Epc:  225,
							Val:  "1",
						},
						{
							Name: "reverse_direction_cumulative_electric_energy",
							Epc:  227,
							Val:  "11",
						},
						{
							Name: "measured_instantaneous",
							Epc:  231,
							Val:  "568",
						},
					},
				},
			}
			devResult := &types.GetDevicesResult{
				StatusCode: 200,
				Devices:    []*types.Device{device},
				Meta: &types.Meta{
					RateLimitLimit:     30.0,
					RateLimitRemaining: 29.0,
					RateLimitReset:     1532778912,
				},
				IsCache: false,
			}
			appResult := &types.GetAppliancesResult{
				StatusCode: 200,
				Appliances: []*types.Appliance{appliance},
				Meta: &types.Meta{
					RateLimitLimit:     30.0,
					RateLimitRemaining: 28.0,
					RateLimitReset:     1532788912,
				},
				IsCache: false,
			}
			remoClient.EXPECT().GetDevices().Return(devResult, nil)
			remoClient.EXPECT().GetAppliances().Return(appResult, nil)

			c, _ := config.NewConfig(mockReader)
			e, err := NewExporter(c, remoClient)
			Expect(err).Should(BeNil())

			ch := make(chan prometheus.Metric)
			defer close(ch)

			go e.Collect(ch)

			m := (<-ch).(prometheus.Metric)
			m2 := readCounter(m)
			Expect(m2.value).To(BeNumerically("==", 50851))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readCounter(m)
			Expect(m2.value).To(BeNumerically("==", 11))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(BeNumerically("==", 1))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(BeNumerically("==", 0.1))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(BeNumerically("==", 6))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(BeNumerically("==", 568))
			Expect(m2.labels["name"]).To(Equal(appliance.Device.Name))
			Expect(m2.labels["id"]).To(Equal(appliance.Device.ID))

			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(appResult.Meta.RateLimitLimit))
			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(appResult.Meta.RateLimitRemaining))
			m = (<-ch).(prometheus.Metric)
			m2 = readGauge(m)
			Expect(m2.value).To(Equal(appResult.Meta.RateLimitReset))

			counter := (<-ch).(prometheus.Counter)
			m2 = readCounter(counter)
			Expect(m2.labels["code"]).To(Equal(strconv.Itoa(devResult.StatusCode)))

			counter = (<-ch).(prometheus.Counter)
			m2 = readCounter(counter)
			Expect(m2.labels["code"]).To(Equal(strconv.Itoa(appResult.StatusCode)))
		})
	})
})
