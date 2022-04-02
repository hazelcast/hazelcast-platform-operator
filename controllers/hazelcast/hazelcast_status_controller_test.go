package hazelcast

import (
	"context"
	"fmt"
	"time"

	"github.com/hazelcast/hazelcast-go-client/cluster"
	hzTypes "github.com/hazelcast/hazelcast-go-client/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/controllers/naming"
)

var _ = Describe("Hazelcast status", func() {
	const (
		timeout  = 10 * time.Second
		interval = 250 * time.Millisecond
	)

	var hzClient = &HazelcastClient{
		MemberMap:            make(map[hzTypes.UUID]*MemberData),
		triggerReconcileChan: make(chan event.GenericEvent),
		NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      n.Hazelcast,
		},
	}

	Context("Hazelcast membership listener", func() {
		It("Should add new member to the map", func() {
			stateChanged := cluster.MembershipStateChanged{
				Member: cluster.MemberInfo{
					Address: cluster.NewAddress("127.0.0.1", 5701),
					UUID:    hzTypes.NewUUID(),
				},
				State: cluster.MembershipStateAdded,
			}
			go getStatusUpdateListener(context.TODO(), hzClient)(stateChanged)

			Eventually(func() bool {
				_, ok := hzClient.MemberMap[stateChanged.Member.UUID]
				return ok
			}, timeout, interval).Should(BeTrue())

			Eventually(func() event.GenericEvent {
				select {
				case e := <-hzClient.triggerReconcileChan:
					return e
				default:
					return event.GenericEvent{}
				}
			}).Should(Equal(event.GenericEvent{
				Object: &v1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
					Namespace: hzClient.NamespacedName.Namespace,
					Name:      hzClient.NamespacedName.Name,
				}}}))
		})

		It("Should remove the existing member from the map", func() {
			existingMember := cluster.MemberInfo{
				Address: cluster.NewAddress("172.10.0.1", 5701),
				UUID:    hzTypes.NewUUID(),
				Version: cluster.MemberVersion{Major: 5, Minor: 0, Patch: 1},
			}
			hzClient.MemberMap[existingMember.UUID] = &MemberData{
				Address: existingMember.Address.String(),
				UUID:    existingMember.UUID.String(),
				Version: fmt.Sprintf(
					"%d.%d.%d", existingMember.Version.Major, existingMember.Version.Minor, existingMember.Version.Patch),
			}

			stateChanged := cluster.MembershipStateChanged{
				Member: existingMember,
				State:  cluster.MembershipStateRemoved,
			}
			go getStatusUpdateListener(context.TODO(), hzClient)(stateChanged)

			Eventually(func() bool {
				_, ok := hzClient.MemberMap[stateChanged.Member.UUID]
				return ok
			}, timeout, interval).Should(BeFalse())

			Eventually(func() event.GenericEvent {
				select {
				case e := <-hzClient.triggerReconcileChan:
					return e
				default:
					return event.GenericEvent{}
				}
			}).Should(Equal(event.GenericEvent{
				Object: &v1alpha1.Hazelcast{ObjectMeta: metav1.ObjectMeta{
					Namespace: hzClient.NamespacedName.Namespace,
					Name:      hzClient.NamespacedName.Name,
				}}}))
		})
	})
})
