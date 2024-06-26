package hazelcast

import (
	"context"
	"fmt"
	"strings"

	hztypes "github.com/hazelcast/hazelcast-go-client/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	"github.com/hazelcast/hazelcast-platform-operator/internal/controller"
	hzclient "github.com/hazelcast/hazelcast-platform-operator/internal/hazelcast-client"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"
)

type HzStatusApplier interface {
	HzStatusApply(hs *hazelcastv1alpha1.HazelcastStatus)
}

type withHzPhase hazelcastv1alpha1.Phase

func (w withHzPhase) HzStatusApply(hs *hazelcastv1alpha1.HazelcastStatus) {
	hs.Phase = hazelcastv1alpha1.Phase(w)
	if hazelcastv1alpha1.Phase(w) == hazelcastv1alpha1.Running {
		hs.Message = ""
	}
}

type withHzFailedPhase string

func (w withHzFailedPhase) HzStatusApply(hs *hazelcastv1alpha1.HazelcastStatus) {
	hs.Phase = hazelcastv1alpha1.Failed
	hs.Message = string(w)
}

type withHzMessage string

func (m withHzMessage) HzStatusApply(hs *hazelcastv1alpha1.HazelcastStatus) {
	hs.Message = string(m)
}

type withHzStatefulSet appsv1.StatefulSet

// HzStatusApply propagates selector and cluster size from underlying StatefulSet
func (s withHzStatefulSet) HzStatusApply(status *hazelcastv1alpha1.HazelcastStatus) {
	// Retrieve the current number of replicas from the StatefulSet
	status.ClusterSize = s.Status.Replicas

	// Retrieve the label selectors from the StatefulSet
	selector, err := metav1.LabelSelectorAsSelector(s.Spec.Selector)
	if err != nil {
		return
	}
	status.Selector = selector.String()
}

type memberStatuses struct {
	readyMembers    string
	readyMembersMap map[hztypes.UUID]*hzclient.MemberData
	restoreState    codecTypes.ClusterHotRestartStatus
	memberPods      []corev1.Pod
	podErrors       util.PodErrors
}

func (r *HazelcastReconciler) withMemberStatuses(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, err error) memberStatuses {
	lk := types.NamespacedName{Name: h.Name, Namespace: h.Namespace}
	ss, ok := r.statusServiceRegistry.Get(lk)
	if !ok {
		return memberStatuses{}
	}
	status := ss.GetStatus()

	readyMembers := "N/A"
	cl, ok := r.clientRegistry.Get(types.NamespacedName{Name: h.Name, Namespace: h.Namespace})
	if ok && cl.IsClientConnected() {
		readyMembers = fmt.Sprintf("%d/%d", len(status.MemberDataMap), *h.Spec.ClusterSize)
	}

	var podErrors util.PodErrors
	if podErrs, isPodErrs := util.AsPodErrors(err); isPodErrs {
		podErrors = podErrs
	}

	var restoreStatus codecTypes.ClusterHotRestartStatus
	if rs := status.ClusterHotRestartStatus.RestoreState(); h.Spec.Persistence.IsEnabled() && rs != hazelcastv1alpha1.RestoreUnknown {
		restoreStatus = status.ClusterHotRestartStatus
	}

	return memberStatuses{
		readyMembers:    readyMembers,
		readyMembersMap: status.MemberDataMap,
		restoreState:    restoreStatus,
		memberPods:      hzMemberPods(ctx, r.Client, h),
		podErrors:       podErrors,
	}
}

func (m memberStatuses) HzStatusApply(hs *hazelcastv1alpha1.HazelcastStatus) {
	hs.Cluster.ReadyMembers = m.readyMembers

	readyMembers := readyMemberStatuses(m.readyMembersMap, m.memberPods)
	failedMembers := failedMemberStatuses(m.podErrors)
	readyFailedMembers := append(readyMembers, failedMembers...)

	pendingMemberPods := pendingMemberPods(m.memberPods, readyFailedMembers)
	pendingMembers := pendingMemberStatuses(pendingMemberPods)

	allMemberStatuses := append(readyFailedMembers, pendingMembers...)
	hs.Members = allMemberStatuses

	if m.restoreState != (codecTypes.ClusterHotRestartStatus{}) {
		hs.Restore = hazelcastv1alpha1.RestoreStatus{
			State:                   m.restoreState.RestoreState(),
			RemainingDataLoadTime:   m.restoreState.RemainingDataLoadTimeSec(),
			RemainingValidationTime: m.restoreState.RemainingValidationTimeSec(),
		}
	}

}

func readyMemberStatuses(m map[hztypes.UUID]*hzclient.MemberData, memberPods []corev1.Pod) []hazelcastv1alpha1.HazelcastMemberStatus {
	members := make([]hazelcastv1alpha1.HazelcastMemberStatus, 0, len(m))
	memberPodIpNameMap := make(map[string]string)
	for _, pod := range memberPods {
		memberPodIpNameMap[pod.Status.PodIP] = pod.Name
	}
	for uid, member := range m {
		ip := strings.Split(member.Address, ":")[0]
		podName := memberPodIpNameMap[ip]
		members = append(members, hazelcastv1alpha1.HazelcastMemberStatus{
			PodName:         podName,
			Uid:             uid.String(),
			Ip:              ip,
			Version:         member.Version,
			Ready:           true,
			Master:          member.Master,
			Lite:            member.LiteMember,
			OwnedPartitions: member.Partitions,
			State:           hazelcastv1alpha1.NodeState(member.MemberState),
		})
	}
	return members
}

func failedMemberStatuses(podErrs util.PodErrors) []hazelcastv1alpha1.HazelcastMemberStatus {
	var statuses []hazelcastv1alpha1.HazelcastMemberStatus
	for _, pErr := range podErrs {
		statuses = append(statuses, hazelcastv1alpha1.HazelcastMemberStatus{
			PodName:      pErr.Name,
			Ip:           pErr.PodIp,
			Ready:        false,
			Message:      pErr.Message,
			Reason:       pErr.Reason,
			RestartCount: pErr.RestartCount,
		})
	}
	return statuses
}

func pendingMemberPods(pods []corev1.Pod, currentMembers []hazelcastv1alpha1.HazelcastMemberStatus) []corev1.Pod {
	memberIps := map[string]struct{}{}
	for _, member := range currentMembers {
		memberIps[member.Ip] = struct{}{}
	}

	var pmPods []corev1.Pod
	for _, pod := range pods {
		if _, exist := memberIps[pod.Status.PodIP]; exist {
			continue
		}
		pmPods = append(pmPods, pod)
	}
	return pmPods
}

func pendingMemberStatuses(pods []corev1.Pod) []hazelcastv1alpha1.HazelcastMemberStatus {
	var statuses []hazelcastv1alpha1.HazelcastMemberStatus
	for _, pod := range pods {
		statuses = append(statuses, hazelcastv1alpha1.HazelcastMemberStatus{
			PodName: pod.Name,
			Ip:      pod.Status.PodIP,
			Ready:   false,
			Message: pod.Status.Message,
			Reason:  pod.Status.Reason,
		})
	}
	return statuses
}

func hzMemberPods(ctx context.Context, c client.Client, h *hazelcastv1alpha1.Hazelcast) []corev1.Pod {
	podList := &corev1.PodList{}
	namespace := client.InNamespace(h.Namespace)
	matchingLabels := client.MatchingLabels(util.Labels(h))
	err := c.List(ctx, podList, namespace, matchingLabels)
	if err != nil {
		return make([]corev1.Pod, 0)
	}
	return podList.Items
}

// update takes the options provided by the given optionsBuilder, applies them all and then updates the Hazelcast resource
func (r *HazelcastReconciler) update(ctx context.Context, h *hazelcastv1alpha1.Hazelcast, recOption controller.ReconcilerOption, options ...HzStatusApplier) (ctrl.Result, error) {
	for _, applier := range options {
		applier.HzStatusApply(&h.Status)
	}

	if err := r.Client.Status().Update(ctx, h); err != nil {
		// Conflicts are expected and will be handled on the next reconcile loop, no need to error out here
		if errors.IsConflict(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if recOption.Err != nil {
		return ctrl.Result{}, recOption.Err
	}
	if h.Status.Phase == hazelcastv1alpha1.Pending {
		return ctrl.Result{Requeue: true, RequeueAfter: recOption.RetryAfter}, nil
	}
	return ctrl.Result{}, nil
}
