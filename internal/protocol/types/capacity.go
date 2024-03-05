package types

type Capacity struct {
	Value int64  `xml:"value,attr"`
	Unit  string `xml:"unit,attr"`
}
