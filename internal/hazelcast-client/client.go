package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	"github.com/hazelcast/hazelcast-go-client"
	proto "github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/cluster"
	hztypes "github.com/hazelcast/hazelcast-go-client/types"
)

type ClientI interface {
	Running() bool
	IsClientConnected() bool
	AreAllMembersAccessible() bool

	OrderedMembers() []cluster.MemberInfo
	InvokeOnMember(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error)
	InvokeOnRandomTarget(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error)

	GetError() error
	Shutdown(ctx context.Context)
}

type Client struct {
	sync.Mutex
	client *hazelcast.Client
	err    error
	log    logr.Logger
}

func NewClient(ctx context.Context, config hazelcast.Config, log logr.Logger) *Client {
	c := &Client{
		log: log,
	}
	go func(ctx context.Context) {
		createHzClient(ctx, c, config)
	}(ctx)
	return c
}

func createHzClient(ctx context.Context, c *Client, config hazelcast.Config) {
	hzClient, err := hazelcast.StartNewClientWithConfig(ctx, config)
	c.Lock()
	defer c.Unlock()
	if err != nil {
		// Ignoring the connection error and just logging as it is expected for Operator that in some scenarios it cannot access the HZ cluster
		c.log.Info("Cannot connect to Hazelcast cluster. Some features might not be available.", "Reason", err.Error())
		c.err = err
	} else {
		c.client = hzClient
	}
}
func (cl *Client) OrderedMembers() []cluster.MemberInfo {
	if cl.client == nil {
		return nil
	}

	icl := hazelcast.NewClientInternal(cl.client)
	return icl.OrderedMembers()
}

func (cl *Client) IsClientConnected() bool {
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

func (cl *Client) AreAllMembersAccessible() bool {
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

func (cl *Client) InvokeOnMember(ctx context.Context, req *proto.ClientMessage, uuid hztypes.UUID, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("Hazelcast client is nil")
	}
	client := cl.client

	ci := hazelcast.NewClientInternal(client)
	return ci.InvokeOnMember(ctx, req, uuid, opts)
}

func (cl *Client) InvokeOnRandomTarget(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
	if cl.client == nil {
		return nil, fmt.Errorf("Hazelcast client is nil")
	}
	client := cl.client

	ci := hazelcast.NewClientInternal(client)
	return ci.InvokeOnRandomTarget(ctx, req, opts)
}

func (cl *Client) Running() bool {
	return cl.client != nil && cl.client.Running()
}

func (cl *Client) GetError() error {
	return cl.err
}

func (c *Client) Shutdown(ctx context.Context) {
	c.Lock()
	defer c.Unlock()

	if c.client == nil {
		return
	}

	if err := c.client.Shutdown(ctx); err != nil {
		c.log.Error(err, "Problem occurred while shutting down the client connection")
	}

}
