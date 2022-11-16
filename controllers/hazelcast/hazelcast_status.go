package hazelcast

import (
	"context"
	"fmt"
	"strings"
	"time"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type optionsBuilder struct {
	phase             hazelcastv1alpha1.Phase
	retryAfter        time.Duration
	err               error
	readyMembers      map[hztypes.UUID]*hzclient.MemberData
	restoreState      codecTypes.ClusterHotRestartStatus
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

func (r *HazelcastReconciler) runningPhaseWithStatus(req ctrl.Request) optionsBuilder {
	if ss, ok := r.statusServiceRegistry.Get(req.NamespacedName); ok {
		return runningPhase().withStatus(ss.GetStatus())
	}
	return runningPhase()
}

func terminatingPhase(err error) optionsBuilder {
	return optionsBuilder{
		phase: hazelcastv1alpha1.Terminating,
		err:   err,
	}
}

func (o optionsBuilder) withStatus(s *hzclient.Status) optionsBuilder {
	o.readyMembers = s.MemberMap
	o.restoreState = s.ClusterHotRestartStatus
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

func statusMembers(m map[hztypes.UUID]*hzclient.MemberData) []hazelcastv1alpha1.HazelcastMemberStatus {
	members := make([]hazelcastv1alpha1.HazelcastMemberStatus, 0, len(m))
	for uid, member := range m {
		a := member.Address
		ip := a[:strings.IndexByte(a, ':')]
		members = append(members, hazelcastv1alpha1.HazelcastMemberStatus{
			Uid:             uid.String(),
			Ip:              ip,
			Version:         member.Version,
			Ready:           true,
			Master:          member.Master,
			Lite:            member.LiteMember,
			OwnedPartitions: member.Partitions,
			State:           member.MemberState,
		})
	}
	return members
}

func addExistingMembers(statusMembers, existingMembers []hazelcastv1alpha1.HazelcastMemberStatus) []hazelcastv1alpha1.HazelcastMemberStatus {
	res := make([]hazelcastv1alpha1.HazelcastMemberStatus, 0, len(statusMembers))
	res = append(res, statusMembers...)
	for _, em := range existingMembers {
		exist := false
		for _, sm := range statusMembers {
			if em.Ip == sm.Ip {
				exist = true
			}
		}
		if !exist {
			res = append(res, em)
		}
	}
	return res
}

func updateFailedMember(h *hazelcastv1alpha1.Hazelcast, err *util.PodError) {
	for _, m := range h.Status.Members {
		if m.Ip == err.PodIp {
			m.PodName = err.Name
			m.Ready = false
			m.Message = err.Message
			m.Reason = err.Reason
			m.RestartCount = err.RestartCount
			return
		}
	}
	h.Status.Members = append(h.Status.Members, hazelcastv1alpha1.HazelcastMemberStatus{
		PodName:      err.Name,
		Ip:           err.PodIp,
		Ready:        false,
		Message:      err.Message,
		Reason:       err.Reason,
		RestartCount: err.RestartCount,
	})
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Hazelcast resource
func (r *HazelcastReconciler) update(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, options optionsBuilder) (ctrl.Result, error) {
	h.Status.Phase = options.phase
	h.Status.Cluster.ReadyMembers = "N/A"

	cl, ok := r.clientRegistry.Get(types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
	if ok && cl.IsClientConnected() {
		h.Status.Cluster.ReadyMembers = fmt.Sprintf("%d/%d", len(options.readyMembers), *h.Spec.ClusterSize)
	}

	h.Status.Message = options.message
	h.Status.ExternalAddresses = options.externalAddresses
	h.Status.Members = addExistingMembers(statusMembers(options.readyMembers), h.Status.Members)
	if options.err != nil {
		if pErr, isPodErr := util.AsPodErrors(options.err); isPodErr {
			for _, podError := range pErr {
				updateFailedMember(h, podError)
			}
		}
	}
	if rs := options.restoreState.RestoreState(); h.Spec.Persistence.IsEnabled() && rs != hazelcastv1alpha1.RestoreUnknown {
		h.Status.Restore = &hazelcastv1alpha1.RestoreStatus{
			State:                   options.restoreState.RestoreState(),
			RemainingDataLoadTime:   options.restoreState.RemainingDataLoadTimeSec(),
			RemainingValidationTime: options.restoreState.RemainingValidationTimeSec(),
		}
	}
	if err := r.Client.Status().Update(ctx, h); err != nil {
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
