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

func (h UsercodeNamespaceService) Apply(ctx context.Context, name string) error {
	filename := name + ".zip"
	request := codec.EncodeDynamicConfigAddUserCodeNamespaceConfigRequest(&types.UserCodeNamespaceConfig{
		Name: name,
		Resources: []types.ResourceDefinition{{
			ID:           filename,
			ResourceType: 5, // JARS_IN_ZIP
			ResourceURL:  "file://" + filepath.Join(n.UserCodeBucketPath, filename),
		}},
	})
	_, err := h.client.InvokeOnRandomTarget(ctx, request, nil)
	return err
}
