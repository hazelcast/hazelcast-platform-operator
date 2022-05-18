package types

type IndexConfig struct {
	Name               string
	Type               int32
	Attributes         []string
	BitmapIndexOptions BitmapIndexOptions
}
