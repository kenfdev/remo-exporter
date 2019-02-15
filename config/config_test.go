package config_test

import (
	"errors"
	"os"
	"strconv"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/mocks"
)

var _ = Describe("Config", func() {

	Describe("NewConfig", func() {
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
		Context("OAUTH_TOKEN and OAUTH_TOKEN_FILE not set", func() {
			It("should fail with error", func() {
				c, err := NewConfig(mockReader)

				Expect(c).To(BeNil())
				Expect(err).NotTo(BeNil())
			})
		})
		Context("OAUTH_TOKEN_FILE set", func() {
			const (
				oAuthTokenFile string = "path/to/token"
			)

			var (
				orgOAuthTokenFile string
			)
			BeforeEach(func() {
				orgOAuthTokenFile = os.Getenv("OAUTH_TOKEN_FILE")

				os.Setenv("OAUTH_TOKEN_FILE", oAuthTokenFile)
			})
			AfterEach(func() {
				os.Setenv("OAUTH_TOKEN_FILE", orgOAuthTokenFile)
			})
			Context("loading file fails", func() {
				It("should fail error", func() {
					// Arrange
					expectedError := errors.New("File not found")

					// Expect
					mockReader.EXPECT().ReadFile(oAuthTokenFile).Return(nil, expectedError)

					// Act
					c, err := NewConfig(mockReader)

					// Assert
					Expect(c).Should(BeNil())
					Expect(err.Error()).Should(ContainSubstring(expectedError.Error()))
				})
			})
			Context("file exists and has token", func() {
				It("should set the token", func() {
					// Arrange
					expectedToken := "some-token"

					// Expect
					mockReader.EXPECT().ReadFile(oAuthTokenFile).Return([]byte(expectedToken), nil)

					// Act
					c, err := NewConfig(mockReader)

					// Assert
					Expect(err).Should(BeNil())
					Expect(c.OAuthToken).Should(Equal(expectedToken))
				})
			})
			Context("file exists but token empty", func() {
				It("should fail with error", func() {
					// Arrange

					// Expect
					mockReader.EXPECT().ReadFile(oAuthTokenFile).Return([]byte(""), nil)

					// Act
					c, err := NewConfig(mockReader)

					// Assert
					Expect(c).Should(BeNil())
					Expect(err).NotTo(BeNil())
				})
			})
		})
		Context("No environment variables except OAUTH_TOKEN set", func() {
			const (
				oAuthToken string = "some_token"
			)

			var (
				orgOAuthToken string
			)
			BeforeEach(func() {
				orgOAuthToken = os.Getenv("OAUTH_TOKEN")

				os.Setenv("OAUTH_TOKEN", oAuthToken)
			})
			AfterEach(func() {
				os.Setenv("OAUTH_TOKEN", orgOAuthToken)
			})
			It("should create a config with default values", func() {
				c, err := NewConfig(mockReader)

				Expect(err).Should(BeNil())
				Expect(c.MetricsPath).To(Equal("/metrics"))
				Expect(c.APIBaseURL).To(Equal("https://api.nature.global"))
				Expect(c.ListenPort).To(Equal("9352"))
				Expect(c.CacheInvalidationSeconds).To(Equal(60))

			})
		})
		Context("Environment variables set", func() {
			const (
				apiBaseURL               string = "https://path.to/somewhere"
				oAuthToken               string = "some_token"
				listenPort               string = "9999"
				cacheInvalidationSeconds string = "30"
				metricsPath              string = "/some/custom/path"
			)

			var (
				orgApiBaseURL               string
				orgOAuthToken               string
				orgListenPort               string
				orgCacheInvalidationSeconds string
				orgMetricsPath              string
			)
			BeforeEach(func() {
				orgApiBaseURL = os.Getenv("API_BASE_URL")
				orgOAuthToken = os.Getenv("OAUTH_TOKEN")
				orgListenPort = os.Getenv("PORT")
				orgCacheInvalidationSeconds = os.Getenv("CACHE_INVALIDATION_SECONDS")
				orgMetricsPath = os.Getenv("METRICS_PATH")

				os.Setenv("API_BASE_URL", apiBaseURL)
				os.Setenv("OAUTH_TOKEN", oAuthToken)
				os.Setenv("PORT", listenPort)
				os.Setenv("CACHE_INVALIDATION_SECONDS", cacheInvalidationSeconds)
				os.Setenv("METRICS_PATH", metricsPath)
			})
			AfterEach(func() {
				os.Setenv("API_BASE_URL", orgApiBaseURL)
				os.Setenv("OAUTH_TOKEN", orgOAuthToken)
				os.Setenv("PORT", orgListenPort)
				os.Setenv("CACHE_INVALIDATION_SECONDS", orgCacheInvalidationSeconds)
				os.Setenv("METRICS_PATH", orgMetricsPath)
			})

			It("should override the default values of the config", func() {
				c, err := NewConfig(mockReader)

				Expect(err).Should(BeNil())
				Expect(c.MetricsPath).To(Equal(metricsPath))
				Expect(c.APIBaseURL).To(Equal(apiBaseURL))
				Expect(c.ListenPort).To(Equal(listenPort))

				secs := strconv.Itoa(c.CacheInvalidationSeconds)
				Expect(secs).To(Equal(cacheInvalidationSeconds))

			})
		})
	})

})
