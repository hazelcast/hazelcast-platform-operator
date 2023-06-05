package main

import (
	"context"
	"fmt"

	"github.com/hazelcast/hazelcast-go-client"
	"github.com/hazelcast/hazelcast-go-client/types"
	"gopkg.in/yaml.v3"

	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
)

type Action int32

const (
	Run    Action = 1
	Get    Action = 2
	Resume Action = 3
	Change Action = 4
)

func main() {
	yamlStr := "config:\n" +
		"  my-key: my-value\n" +
		"  another-key: 3\n" +
		"seriazilation:\n" +
		"  disabled: true"

	dynamic := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlStr), dynamic)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", dynamic)
	if 1 == 1 {
		return
	}
	action := Change
	ctx := context.Background()
	config := hazelcast.NewConfig()
	config.Cluster.Network.SetAddresses("127.0.0.1:5701")
	config.Cluster.Unisocket = true
	c, _ := hazelcast.StartNewClientWithConfig(ctx, config)
	ci := hazelcast.NewClientInternal(c)
	uid := types.UUID{}
	for _, info := range ci.OrderedMembers() {
		if info.Address.String() == "10.16.2.8:5701" {
			uid = info.UUID
		}
	}
	var jobId int64
	jobId = 716240036697210883

	switch action {
	case Run:
		runJob(ctx, ci, uid)
	case Get:
		getJobs(ctx, ci, uid)
	case Resume:
		resumeJob(ctx, jobId, ci, uid)
	case Change:
		changeJob(ctx, jobId, ci, uid)
	}
}

func changeJob(ctx context.Context, jobId int64, ci *hazelcast.ClientInternal, uid types.UUID) {
	terminateJob := codecTypes.JetTerminateJob{
		JobId:         jobId,
		TerminateMode: codecTypes.CancelForcefully,
	}
	req := codec.EncodeJetTerminateJobRequest(terminateJob)
	_, err := ci.InvokeOnMember(ctx, req, uid, nil)
	if err != nil {
		panic(err)
	}
}

func resumeJob(ctx context.Context, jobId int64, ci *hazelcast.ClientInternal, uid types.UUID) {
	req := codec.EncodeJetResumeJobRequest(jobId)
	_, err := ci.InvokeOnMember(ctx, req, uid, nil)
	if err != nil {
		panic(err)
	}
}

func getJobs(ctx context.Context, ci *hazelcast.ClientInternal, uid types.UUID) {
	req := codec.EncodeJetGetJobAndSqlSummaryListRequest()
	resp, err := ci.InvokeOnMember(ctx, req, uid, nil)
	if err != nil {
		panic(err)
	}
	lr := codec.DecodeJetGetJobAndSqlSummaryListResponse(resp)
	for _, summary := range lr {
		fmt.Printf("%s -> %d", summary.NameOrId, summary.JobId)
	}
}

func runJob(ctx context.Context, ci *hazelcast.ClientInternal, uid types.UUID) {
	md := codecTypes.DefaultExistingJarJobMetaData("my-job", "/opt/hazelcast/jetJobJars/jet-pipeline-longrun-2.0.0.jar")
	request := codec.EncodeJetUploadJobMetaDataRequest(md)
	_, err := ci.InvokeOnMember(ctx, request, uid, nil)
	if err != nil {
		panic(err)
	}
	return
}
