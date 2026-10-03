package exporter_test

import (
	"encoding/json"
	"testing"

	"github.com/kenfdev/remo-exporter/config"
	"github.com/kenfdev/remo-exporter/exporter"
	"github.com/kenfdev/remo-exporter/types"
	"github.com/prometheus/client_golang/prometheus"
)

type fixtureGatherer struct {
	devices []*types.Device
}

func (g fixtureGatherer) GetDevices() (*types.GetDevicesResult, error) {
	return &types.GetDevicesResult{Devices: g.devices}, nil
}

func (fixtureGatherer) GetAppliances() (*types.GetAppliancesResult, error) {
	return &types.GetAppliancesResult{}, nil
}

func TestDeviceOnlineMetric(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		present bool
		want    float64
	}{
		{"online", `{"id":"device-1","name":"Living room","online":true}`, true, 1},
		{"offline", `{"id":"device-1","name":"Living room","online":false}`, true, 0},
		{"null", `{"id":"device-1","name":"Living room","online":null}`, false, 0},
		{"absent", `{"id":"device-1","name":"Living room"}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var device types.Device
			if err := json.Unmarshal([]byte(tc.body), &device); err != nil {
				t.Fatal(err)
			}
			e, err := exporter.NewExporter(&config.Config{}, fixtureGatherer{[]*types.Device{&device}})
			if err != nil {
				t.Fatal(err)
			}
			registry := prometheus.NewPedanticRegistry()
			if err := registry.Register(e); err != nil {
				t.Fatal(err)
			}
			families, err := registry.Gather()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, family := range families {
				if family.GetName() != "remo_device_online" {
					continue
				}
				found = true
				if len(family.Metric) != 1 {
					t.Fatalf("got %d samples, want 1", len(family.Metric))
				}
				m := family.Metric[0]
				if m.GetGauge().GetValue() != tc.want {
					t.Fatalf("got %v, want %v", m.GetGauge().GetValue(), tc.want)
				}
				labels := labels2Map(m.GetLabel())
				if len(labels) != 2 || labels["name"] != "Living room" || labels["id"] != "device-1" {
					t.Fatalf("unexpected labels: %v", labels)
				}
			}
			if found != tc.present {
				t.Fatalf("online metric present = %v, want %v", found, tc.present)
			}
		})
	}
}
