package types

type ResourceDefinition struct {
	ID           string
	ResourceType int32
	Payload      []byte
	ResourceURL  string
}
