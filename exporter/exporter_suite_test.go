package exporter_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestExporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exporter Suite")
}

const (
	oAuthToken string = "some_token"
)

var (
	orgOAuthToken string
)

var _ = BeforeSuite(func() {
	orgOAuthToken = os.Getenv("OAUTH_TOKEN")

	os.Setenv("OAUTH_TOKEN", oAuthToken)
})

var _ = AfterSuite(func() {
	os.Setenv("OAUTH_TOKEN", orgOAuthToken)

})
