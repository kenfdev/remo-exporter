package types_test

import (
	"encoding/json"
	"testing"

	"github.com/kenfdev/remo-exporter/types"
)

func TestDeviceTemperatureOffsetAcceptsFractions(t *testing.T) {
	var device types.Device
	if err := json.Unmarshal([]byte(`{"temperature_offset":-0.5}`), &device); err != nil {
		t.Fatal(err)
	}
	if got := float64(device.TemperatureOffset); got != -0.5 {
		t.Fatalf("temperature offset = %v, want -0.5", got)
	}
}

func TestDeviceOnlinePreservesUnknownAndBooleanValues(t *testing.T) {
	online, offline := true, false
	for _, test := range []struct {
		name string
		body string
		want *bool
	}{
		{name: "absent", body: `{}`, want: nil},
		{name: "null", body: `{"online":null}`, want: nil},
		{name: "offline", body: `{"online":false}`, want: &offline},
		{name: "online", body: `{"online":true}`, want: &online},
	} {
		t.Run(test.name, func(t *testing.T) {
			var device types.Device
			if err := json.Unmarshal([]byte(test.body), &device); err != nil {
				t.Fatal(err)
			}
			if test.want == nil {
				if device.Online != nil {
					t.Fatalf("online = %v, want unknown", *device.Online)
				}
				return
			}
			if device.Online == nil || *device.Online != *test.want {
				t.Fatalf("online = %v, want %v", device.Online, *test.want)
			}
		})
	}
}
