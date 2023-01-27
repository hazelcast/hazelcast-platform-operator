package kubeclient

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// defaultClient is the default Kubernetes Client
var defaultClient client.Reader = &nopClient{}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
// obj must be a struct pointer so that obj can be updated with the response
// returned by the Server.
func Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return defaultClient.Get(ctx, key, obj, opts...)
}

// List retrieves list of objects for a given namespace and list options. On a
// successful call, Items field in the list will be populated with the
// result returned from the server.
func List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return defaultClient.List(ctx, list, opts...)
}

func Setup(c client.Client) manager.Runnable {
	return &runnableSetup{Client: c}
}

type runnableSetup struct {
	client.Client
}

func (c *runnableSetup) Start(ctx context.Context) error {
	defaultClient = c.Client
	return nil
}

// nopClient is a no operation client used as a default before setup()
type nopClient struct{}

func (c *nopClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

func (c *nopClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}
