package client

import (
	"context"
	"fmt"

	"github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
)

type Client interface {
	Running() bool
	IsClientConnected() bool
	AreAllMembersAccessible() bool

	OrderedMembers() []cluster.MemberInfo
	InvokeOnMember(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error)
	InvokeOnRandomTarget(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error)

	Shutdown(ctx context.Context) error
}

type HazelcastClient struct {
	client *hazelcast.Client
}

func NewClient(ctx context.Context, config hazelcast.Config) (*HazelcastClient, error) {
	c := &HazelcastClient{}
	hzcl, err := createHzClient(ctx, config)
	if err != nil {
		return nil, err
	}
	c.client = hzcl
	return c, nil
}

func createHzClient(ctx context.Context, config hazelcast.Config) (*hazelcast.Client, error) {
	hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
	if err != nil {
		// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
		return nil, err
	}
	return hzClient, nil
}
func (cl *HazelcastClient) OrderedMembers() []cluster.MemberInfo {
	if cl.client == nil {
		return nil
	}

	icl := hazelcast.NewClientInternal(cl.client)
	return icl.OrderedMembers()
}

func (cl *HazelcastClient) IsClientConnected() bool {
	if cl.client == nil {
		return false
	}

	icl := hazelcast.NewClientInternal(cl.client)
	for _, mem := range icl.OrderedMembers() {
		if icl.ConnectedToMember(mem.UUID) {
			return true
		}
	}
	return false
}

func (cl *HazelcastClient) AreAllMembersAccessible() bool {
	if cl.client == nil {
		return false
	}

	icl := hazelcast.NewClientInternal(cl.client)
	for _, mem := range icl.OrderedMembers() {
		if !icl.ConnectedToMember(mem.UUID) {
			return false
		}
	}
	return true
}

func (cl *HazelcastClient) InvokeOnMember(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("Hazelcast client is nil")
	}
	client := cl.client

	ci := hazelcast.NewClientInternal(client)
	return ci.InvokeOnMember(ctx, req, uuid, opts)
}

func (cl *HazelcastClient) InvokeOnRandomTarget(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("Hazelcast client is nil")
	}
	client := cl.client

	ci := hazelcast.NewClientInternal(client)
	return ci.InvokeOnRandomTarget(ctx, req, opts)
}

func (cl *HazelcastClient) Running() bool {
	return cl.client != nil && cl.client.Running()
}

func (c *HazelcastClient) Shutdown(ctx context.Context) error {
	if c.client == nil {
		return nil
	}

	if err := c.client.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
