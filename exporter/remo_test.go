package exporter_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/kenfdev/remo-exporter/config"
	. "github.com/kenfdev/remo-exporter/exporter"
	"github.com/kenfdev/remo-exporter/mocks"
)

var _ = Describe("Remo", func() {
	Describe("GetDevices", func() {
		var (
			mockCtrl   *gomock.Controller
			mockReader config.Reader
		)
		const (
			sampleJson = `
[
		{
				"name": "Living Remo",
				"id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				"created_at": "2018-01-01T00:00:00Z",
				"updated_at": "2018-01-02T00:00:00Z",
				"firmware_version": "Remo/1.0.62-gabbf5bd",
				"temperature_offset": 0,
				"humidity_offset": 0,
				"users": [
						{
								"id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
								"nickname": "John Doe",
								"superuser": true
						}
				],
				"newest_events": {
						"hu": {
								"val": 50,
								"created_at": "2018-01-05T00:00:00Z"
						},
						"il": {
								"val": 25.2,
								"created_at": "2018-01-05T00:00:00Z"
						},
						"te": {
								"val": 27.59,
								"created_at": "2018-01-05T00:00:00Z"
						}
				}
		}
]
`
		)
		BeforeEach(func() {
			mockCtrl = gomock.NewController(GinkgoT())
			mockReader = mocks.NewMockReader(mockCtrl)
		})
		AfterEach(func() {
			mockCtrl.Finish()
		})
		Context("successful request", func() {
			It("should return the devices recieved in the response", func() {

				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				response := &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(sampleJson)),
				}

				authClient.EXPECT().Get(gomock.Any()).Return(response, nil)

				c, _ := config.NewConfig(mockReader)
				rc, _ := NewRemoClient(c, authClient)

				result, err := rc.GetDevices()

				Expect(err).Should(BeNil())
				Expect(len(result.Devices)).To(BeNumerically(">", 0))

				device := result.Devices[0]
				Expect(device.Name).To(Equal("Living Remo"))
				Expect(device.NewestEvents.Humidity.Value).To(Equal(float64(50)))
				Expect(device.NewestEvents.Illumination.Value).To(Equal(float64(25.2)))
				Expect(device.NewestEvents.Temperature.Value).To(Equal(float64(27.59)))

			})

			It("should return a cached result if there is a valid cache", func() {
				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				response := &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(sampleJson)),
					Header:     make(http.Header, 0),
				}
				response.Header.Set("X-Rate-Limit-Limit", "30")
				response.Header.Set("X-Rate-Limit-Remaining", "29")
				response.Header.Set("X-Rate-Limit-Reset", "1532778912")

				authClient.EXPECT().Get(gomock.Any()).Return(response, nil).Times(1)

				c, _ := config.NewConfig(mockReader)
				rc, _ := NewRemoClient(c, authClient)

				firstResponse, err := rc.GetDevices()
				Expect(err).Should(BeNil())

				secondResponse, err := rc.GetDevices()
				Expect(err).Should(BeNil())

				Expect(firstResponse.Meta).To(Equal(secondResponse.Meta))
				Expect(firstResponse.StatusCode).To(Equal(secondResponse.StatusCode))
				Expect(firstResponse.Devices).To(Equal(secondResponse.Devices))

				Expect(firstResponse.IsCache).To(BeFalse())
				Expect(secondResponse.IsCache).To(BeTrue())
			})

			It("should fetch new data if the cache is invalidated", func() {
				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				response1 := &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(sampleJson)),
				}
				response2 := &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString("[]")),
				}

				gomock.InOrder(
					authClient.EXPECT().Get(gomock.Any()).Return(response1, nil).Times(1),
					authClient.EXPECT().Get(gomock.Any()).Return(response2, nil).Times(1),
				)

				c, _ := config.NewConfig(mockReader)
				c.CacheInvalidationSeconds = 0 // invalidate the cache immediately

				rc, _ := NewRemoClient(c, authClient)

				firstResponse, err := rc.GetDevices()
				Expect(err).Should(BeNil())

				secondResponse, err := rc.GetDevices()
				Expect(err).Should(BeNil())

				Expect(firstResponse).NotTo(Equal(secondResponse))
			})
		})
		Context("request failure", func() {
			It("should return the error", func() {
				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				expectedError := errors.New("Invalid Request")
				authClient.EXPECT().Get(gomock.Any()).Return(nil, expectedError).Times(1)

				c, _ := config.NewConfig(mockReader)

				rc, _ := NewRemoClient(c, authClient)

				response, err := rc.GetDevices()
				Expect(response).Should(BeNil())
				Expect(err).Should(Equal(expectedError))
			})
			It("should return stats even though the response status code isn't 200", func() {
				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				response := &http.Response{
					StatusCode: 401,
					Body:       ioutil.NopCloser(bytes.NewBufferString("{}")),
				}

				authClient.EXPECT().Get(gomock.Any()).Return(response, nil).Times(1)

				c, _ := config.NewConfig(mockReader)

				rc, _ := NewRemoClient(c, authClient)

				result, err := rc.GetDevices()
				Expect(err).Should(BeNil())
				Expect(result.StatusCode).Should(Equal(response.StatusCode))

			})
		})
	})

	Describe("GetAppliances", func() {
		var (
			mockCtrl   *gomock.Controller
			mockReader config.Reader
		)
		const (
			sampleJson = `
[
	{
		"id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		"device": {
			"name": "Remo E lite",
			"id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			"created_at": "2020-05-13T02:23:18Z",
			"updated_at": "2020-05-13T02:27:16Z",
			"mac_address": "xx:xx:xx:xx:xx:xx",
			"bt_mac_address": "xx:xx:xx:xx:xx:xx",
			"serial_number": "XXXXXXXXXXXXXX",
			"firmware_version": "Remo-E-lite/1.1.2",
			"temperature_offset": 0,
			"humidity_offset": 0
		},
		"model": {
			"id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
			"manufacturer": "",
			"name": "Smart Meter",
			"image": "ico_smartmeter"
		},
		"type": "EL_SMART_METER",
		"nickname": "スマートメーター",
		"image": "ico_smartmeter",
		"settings": null,
		"aircon": null,
		"signals": [],
		"smart_meter": {
			"echonetlite_properties": [
				{
					"name": "coefficient",
					"epc": 211,
					"val": "1",
					"updated_at": "2020-05-20T10:42:21Z"
				},
				{
					"name": "cumulative_electric_energy_effective_digits",
					"epc": 215,
					"val": "6",
					"updated_at": "2020-05-20T10:42:21Z"
				},
				{
					"name": "normal_direction_cumulative_electric_energy",
					"epc": 224,
					"val": "50851",
					"updated_at": "2020-05-20T10:42:21Z"
				},
				{
					"name": "cumulative_electric_energy_unit",
					"epc": 225,
					"val": "1",
					"updated_at": "2020-05-20T10:42:21Z"
				},
				{
					"name": "reverse_direction_cumulative_electric_energy",
					"epc": 227,
					"val": "11",
					"updated_at": "2020-05-20T10:42:21Z"
				},
				{
					"name": "measured_instantaneous",
					"epc": 231,
					"val": "568",
					"updated_at": "2020-05-20T10:42:21Z"
				}
			]
		}
	}
]
`
		)
		BeforeEach(func() {
			mockCtrl = gomock.NewController(GinkgoT())
			mockReader = mocks.NewMockReader(mockCtrl)
		})
		AfterEach(func() {
			mockCtrl.Finish()
		})
		Context("successful request", func() {
			It("should return the appliances received in the response", func() {

				authClient := mocks.NewMockAuthHttpDoer(mockCtrl)

				response := &http.Response{
					StatusCode: 200,
					Body:       ioutil.NopCloser(bytes.NewBufferString(sampleJson)),
				}

				authClient.EXPECT().Get(gomock.Any()).Return(response, nil)

				c, _ := config.NewConfig(mockReader)
				rc, _ := NewRemoClient(c, authClient)

				result, err := rc.GetAppliances()

				Expect(err).Should(BeNil())
				Expect(len(result.Appliances)).To(BeNumerically(">", 0))

				app := result.Appliances[0]
				Expect(app.Type).To(Equal("EL_SMART_METER"))
				Expect(app.Device.Name).To(Equal("Remo E lite"))
				Expect(app.SmartMeter.EchonetliteProperties[2].Epc).To(Equal(EpcNormalDirectionCumulativeElectricEnergy))
				Expect(app.SmartMeter.EchonetliteProperties[2].Val).To(Equal("50851"))
			})
		})
	})
})
