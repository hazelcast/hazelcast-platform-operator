package client

import (
	"context"
	"path/filepath"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"

	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
)

type UsercodeNamespaceService struct {
	client Client
}

func NewUsercodeNamespaceService(client Client) *UsercodeNamespaceService {
	return &UsercodeNamespaceService{
		client: client,
	}
}

type ucnResourceType int32

const (
	class ucnResourceType = iota + 3
	jar
	jarsInZip
)

func (h UsercodeNamespaceService) Apply(ctx context.Context, name string) error {
	filename := name + ".zip"
	request := codec.EncodeDynamicConfigAddUserCodeNamespaceConfigRequest(&types.UserCodeNamespaceConfig{
		Name: name,
		Resources: []types.ResourceDefinition{{
			ID:           filename,
			ResourceType: int32(jarsInZip),
			ResourceURL:  "file://" + filepath.Join(n.UCNBucketPath, filename),
		}},
	})
	for _, m := range h.client.OrderedMembers() {
		_, err := h.client.InvokeOnMember(ctx, request, m.UUID, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
