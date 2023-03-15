package main

import (
	"context"

	"github.com/hazelcast/hazelcast-go-client"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

func main() {
	ctx := context.Background()
	config := hazelcast.Config{}
	config.Cluster.Unisocket = true
	config.Cluster.Network.Addresses = []string{"34.78.106.160:5701"}
	c, _ := hazelcast.StartNewClientWithConfig(ctx, config)
	ci := hazelcast.NewClientInternal(c)
	for _, m := range ci.OrderedMembers() {
		println(m.String())
	}

	md := types.JobMetaData{}
	md.JobName = "myjob"
	md.JarOnMember = true
	md.FileName = "/opt/hazelcast/userCode/bucket/jet-pipeline-1.0.2.jar"
	request := codec.EncodeJetUploadJobMetaDataRequest(md)
	_, err := ci.InvokeOnRandomTarget(ctx, request, nil)
	if err != nil {
		panic(err)
	}
}
