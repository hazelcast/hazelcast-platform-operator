package types

type IndexConfig struct {
	Name               string
	Type               IndexType
	Attributes         []string
	BitmapIndexOptions BitmapIndexOptions
}
