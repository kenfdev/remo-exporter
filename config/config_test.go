package config_test

import (
	"os"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/kenfdev/remo-exporter/config"
)

var _ = Describe("Config", func() {

	Describe("NewConfig", func() {
		Context("OAUTH_TOKEN not set", func() {
			It("should fail with error", func() {
				c, err := NewConfig()

				Expect(c).To(BeNil())
				Expect(err).NotTo(BeNil())
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
				c, err := NewConfig()

				Expect(err).Should(BeNil())
				Expect(c.MetricsPath).To(Equal("/metrics"))
				Expect(c.APIBaseURL).To(Equal("https://api.nature.global"))
				Expect(c.ListenPort).To(Equal("9470"))
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
				c, err := NewConfig()

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
