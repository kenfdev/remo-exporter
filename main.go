package main

import (
	"net/http"
	"os"

	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/exporter"
	authHttp "github.com/kenfdev/remo-exporter/http"
	"github.com/kenfdev/remo-exporter/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {

	log.Info("Starting Nature Remo Exporter")
	r := config.NewFileReader()
	c, err := config.NewConfig(r)
	if err != nil {
		log.Errorf("Failed to create config: %v", err)
		os.Exit(1)
	}

	authClient := authHttp.NewAuthHttpClient(c.OAuthToken)

	rc, err := exporter.NewRemoClient(c, authClient)
	if err != nil {
		log.Errorf("Failed to create remo client: %v", err)
		os.Exit(1)
	}

	e, err := exporter.NewExporter(c, rc)
	if err != nil {
		log.Errorf("Failed to create exporter: %v", err)
		os.Exit(1)
	}

	prometheus.MustRegister(e)

	http.Handle(c.MetricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
		                <head><title>Nature Remo Exporter</title></head>
		                <body>
		                   <h1>Nature Remo Prometheus Metrics Exporter</h1>
						   <p>For more information, visit <a href=https://github.com/kenfdev/remo-exporter>GitHub</a></p>
		                   <p><a href='` + c.MetricsPath + `'>Metrics</a></p>
		                   </body>
		                </html>
		              `))
	})
	log.Infof("Listening on :%s", c.ListenPort)
	log.Fatal(http.ListenAndServe(":"+c.ListenPort, nil))
}
