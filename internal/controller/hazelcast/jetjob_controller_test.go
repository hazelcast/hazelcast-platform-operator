package hazelcast

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	proto "github.com/hazelcast/hazelcast-go-client"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/protocol/codec"
)

func Test_ShouldSendParametersInRequest(t *testing.T) {
	RegisterFailHandler(fail(t))

	jj := &hazelcastv1alpha1.JetJob{
		ObjectMeta: v1.ObjectMeta{
			Name: "jet-job",
		},
		Spec: hazelcastv1alpha1.JetJobSpec{
			State:      hazelcastv1alpha1.RunningJobState,
			Parameters: []string{"param1", "secondParameter", "3rdParam"},
		},
		Status: hazelcastv1alpha1.JetJobStatus{
			Phase: hazelcastv1alpha1.JetJobNotRunning,
		},
	}
	c := &fakeHzClient{}
	var params []string
	c.tInvokeOnRandomTarget = func(ctx context.Context, req *proto.ClientMessage, opts *proto.InvokeOptions) (*proto.ClientMessage, error) {
		if req.Type() == codec.JetUploadJobMetaDataCodecRequestMessageType {
			jobMetaData := codec.DecodeJetUploadJobMetaDataRequest(req)
			params = jobMetaData.JobParameters
		}
		clientMessage := proto.NewClientMessageForEncode()
		initialFrame := proto.NewFrameWith(make([]byte, 0), proto.UnfragmentedMessage)
		clientMessage.AddFrame(initialFrame)
		clientMessage.AddFrame(initialFrame)
		endFrame := proto.NewFrameWith(make([]byte, 0), proto.EndDataStructureFlag)
		clientMessage.AddFrame(endFrame)
		return clientMessage, nil
	}
	nn := types.NamespacedName{Namespace: "default", Name: "hazelcast"}
	clReg := &fakeHzClientRegistry{}
	clReg.Set(nn, c)
	r := &JetJobReconciler{
		ClientRegistry: clReg,
		Client:         fakeK8sClient(jj),
	}
	_, err := r.applyJetJob(context.TODO(), jj, nn, logr.Discard())
	Expect(err).To(BeNil())
	Expect(params).To(ContainElements(jj.Spec.Parameters))
}
