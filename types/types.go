package types

type User struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	Superuser bool   `json:"superuser"`
}

type SensorValue struct {
	Value     float64 `json:"val"`
	CreatedAt string  `json:"created_at"`
}
type Event struct {
	Temperature  *SensorValue `json:"te"`
	Humidity     *SensorValue `json:"hu"`
	Illumination *SensorValue `json:"il"`
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
