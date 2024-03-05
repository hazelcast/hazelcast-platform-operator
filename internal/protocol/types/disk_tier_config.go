package types

type DiskTierConfig struct {
	Enabled    bool   `xml:"enabled,attr"`
	DeviceName string `xml:"device-name,attr"`
}
