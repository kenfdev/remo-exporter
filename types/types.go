package types

import (
	"strings"
	"time"
)

type User struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	Superuser bool   `json:"superuser"`
}

type SensorValue struct {
	Value     float64   `json:"val"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Temperature  *SensorValue `json:"te"`
	Humidity     *SensorValue `json:"hu"`
	Illumination *SensorValue `json:"il"`
	Motion       *SensorValue `json:"mo"`
}

type Device struct {
	Name              string  `json:"name"`
	ID                string  `json:"id"`
	CreatedAt         string  `json:"created_at"`
	UpdatedAt         string  `json:"updated_at"`
	FirmwareVersion   string  `json:"firmware_version"`
	TemperatureOffset int     `json:"temperature_offset"`
	HumidityOffset    int     `json:"humidity_offset"`
	Users             []*User `json:"users"`
	NewestEvents      *Event  `json:"newest_events"`
}

func (d Device) IsRemo() bool {
	return strings.HasPrefix(d.FirmwareVersion, "Remo/")
}

func (d Device) IsRemoElite() bool {
	return strings.HasPrefix(d.FirmwareVersion, "Remo-E-lite/")
}

type Meta struct {
	RateLimitLimit     float64
	RateLimitReset     float64
	RateLimitRemaining float64
}

// GetDevicesResult is the result of invoking the Remo API
type GetDevicesResult struct {
	StatusCode int
	Meta       *Meta
	Devices    []*Device
	IsCache    bool
}

type GetAppliancesResult struct {
	StatusCode int
	Meta       *Meta
	Appliances []*Appliance
	IsCache    bool
}

type Appliance struct {
	ID         string      `json:"id"`
	Device     *Device     `json:"device"`
	Model      *Model      `json:"model"`
	Type       string      `json:"type"`
	Nickname   string      `json:"nickname"`
	Image      string      `json:"image"`
	SmartMeter *SmartMeter `json:"smart_meter"`
}

type Model struct {
	ID           string `json:"id"`
	Manufacturer string `json:"manufacturer"`
	Name         string `json:"name"`
	Image        string `json:"image"`
}

type SmartMeter struct {
	EchonetliteProperties []*EchonetliteProperty `json:"echonetlite_properties"`
}

type EchonetliteProperty struct {
	Name      string    `json:"name"`
	Epc       int       `json:"epc"`
	Val       string    `json:"val"`
	UpdatedAt time.Time `json:"updated_at"`
}
