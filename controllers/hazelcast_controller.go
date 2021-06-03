package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-enterprise-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// HazelcastReconciler reconciles a Hazelcast object
type HazelcastReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=hazelcast.com,resources=hazelcasts/finalizers,verbs=update

func (r *HazelcastReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("hazelcast", req.NamespacedName)

	h := &hazelcastv1alpha1.Hazelcast{}
	err := r.Client.Get(ctx, req.NamespacedName, h)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Hazelcast resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Hazelcast")
		return ctrl.Result{}, err
	}

	// Check if the statefulSet already exists, if not create a new one
	found := &appsv1.StatefulSet{}
	err = r.Get(ctx, types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, found)
	expected, err2 := r.statefulSetForHazelcast(h)
	if err2 != nil {
		logger.Error(err, "Failed to create new StatefulSet resource")
		return ctrl.Result{}, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
		err = r.Create(ctx, expected)
		if err != nil {
			logger.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get StatefulSet")
		return ctrl.Result{}, err
	}

	// TODO Find a better comparison mechanism. Currently found object has so much default props.
	if !apiequality.Semantic.DeepEqual(found.Spec, expected.Spec) {
		logger.Info("Updating a StatefulSet", "StatefulSet.Namespace", expected.Namespace, "StatefulSet.Name", expected.Name)
		if err := r.Update(ctx, expected); err != nil {
			logger.Error(err, "Failed to update StatefulSet")
		}
	}

	return ctrl.Result{}, nil
}

func (r *HazelcastReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hazelcastv1alpha1.Hazelcast{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}

func (r *HazelcastReconciler) statefulSetForHazelcast(h *hazelcastv1alpha1.Hazelcast) (*appsv1.StatefulSet, error) {
	ls := labelsForHazelcast(h)
	replicas := h.Spec.ClusterSize

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name,
			Namespace: h.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: ImageForCluster(h),
						Name:  "hazelcast",
						Ports: []v1.ContainerPort{{
							ContainerPort: 5701,
							Name:          "hazelcast",
						}},
						Env: []v1.EnvVar{
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_ENABLED",
								Value: "true",
							},
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_PODLABELNAME",
								Value: "app.kubernetes.io/instance",
							},
							{
								Name:  "HZ_NETWORK_JOIN_KUBERNETES_PODLABELVALUE",
								Value: h.Name,
							},
						},
					}},
				},
			},
		},
	}
	// Set Hazelcast instance as the owner and controller
	err := ctrl.SetControllerReference(h, sts, r.Scheme)
	if err != nil {
		return nil, err
	}

	return sts, nil
}

func labelsForHazelcast(h *hazelcastv1alpha1.Hazelcast) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "hazelcast",
		"app.kubernetes.io/instance":   h.Name,
		"app.kubernetes.io/managed-by": "hazelcast-enterprise-operator",
	}
}

func ImageForCluster(h *hazelcastv1alpha1.Hazelcast) string {
	return fmt.Sprintf("%s:%s", h.Spec.Repository, h.Spec.Version)
}
