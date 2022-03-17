package hazelcast

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hazelcast/hazelcast-go-client/cluster"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
)

type optionsBuilder struct {
	phase             hazelcastv1alpha1.Phase
	retryAfter        time.Duration
	err               error
	readyMembers      map[string]cluster.MemberInfo
	message           string
	externalAddresses string
}

func failedPhase(err error) optionsBuilder {
	return optionsBuilder{
		phase: hazelcastv1alpha1.Failed,
		err:   err,
	}
}

func pendingPhase(retryAfter time.Duration) optionsBuilder {
	return optionsBuilder{
		phase:      hazelcastv1alpha1.Pending,
		retryAfter: retryAfter,
	}
}

func runningPhase() optionsBuilder {
	return optionsBuilder{
		phase: hazelcastv1alpha1.Running,
	}
}

func (o optionsBuilder) withReadyMembers(m map[string]cluster.MemberInfo) optionsBuilder {
	o.readyMembers = m
	return o
}

func (o optionsBuilder) withMessage(m string) optionsBuilder {
	o.message = m
	return o
}

func (o optionsBuilder) withExternalAddresses(addrs string) optionsBuilder {
	o.externalAddresses = addrs
	return o
}

func statusMembers(m map[string]cluster.MemberInfo) []hazelcastv1alpha1.HazelcastMemberStatus {
	members := make([]hazelcastv1alpha1.HazelcastMemberStatus, 0, len(m))
	for uid, member := range m {
		a := member.Address.String()
		members = append(members, hazelcastv1alpha1.HazelcastMemberStatus{
			Uid:     uid,
			Ip:      a[:strings.IndexByte(a, ':')],
			Version: fmt.Sprintf("%d.%d.%d", member.Version.Major, member.Version.Minor, member.Version.Patch),
			Ready:   true,
		})
	}
	return members
}

func updateFailedMember(h *hazelcastv1alpha1.Hazelcast, err *util.PodError) {
	for _, m := range h.Status.Members {
		if m.Ip == err.PodIp {
			m.PodName = err.Name
			m.Ready = false
			m.Message = err.Message
			m.Reason = err.Reason
			return
		}
	}
	h.Status.Members = append(h.Status.Members, hazelcastv1alpha1.HazelcastMemberStatus{
		PodName: err.Name,
		Ip:      err.PodIp,
		Ready:   false,
		Message: err.Message,
		Reason:  err.Reason,
	})
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Hazelcast resource
func update(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast, options optionsBuilder) (ctrl.Result, error) {
	h.Status.Phase = options.phase
	h.Status.Cluster.ReadyMembers = fmt.Sprintf("%d/%d", len(options.readyMembers), *h.Spec.ClusterSize)
	h.Status.Message = options.message
	h.Status.ExternalAddresses = options.externalAddresses
	h.Status.Members = statusMembers(options.readyMembers)
	if options.err != nil {
		if pErr, isPodErr := util.AsPodErrors(options.err); isPodErr {
			for _, podError := range pErr {
				updateFailedMember(h, podError)
			}
		}
	}
	if err := c.Status().Update(ctx, h); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if options.phase == hazelcastv1alpha1.Failed {
		return ctrl.Result{}, options.err
	}
	if options.phase == hazelcastv1alpha1.Pending {
		return ctrl.Result{Requeue: true, RequeueAfter: options.retryAfter}, nil
	}
	return ctrl.Result{}, nil
}
