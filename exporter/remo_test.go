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
			mockCtrl *gomock.Controller
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

				c, _ := config.NewConfig()
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

				c, _ := config.NewConfig()
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

				c, _ := config.NewConfig()
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

				c, _ := config.NewConfig()

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

				c, _ := config.NewConfig()

				rc, _ := NewRemoClient(c, authClient)

				result, err := rc.GetDevices()
				Expect(err).Should(BeNil())
				Expect(result.StatusCode).Should(Equal(response.StatusCode))

			})
		})
	})

})
