package turbine

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

var (
	PingPong = func(ns string) *hazelcastv1alpha1.Turbine {
		return &hazelcastv1alpha1.Turbine{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ping-pong",
			},
			Spec: hazelcastv1alpha1.TurbineSpec{
				Sidecar: hazelcastv1alpha1.SidecarConfiguration{
					Name:  "turbine-sidecar",
					Image: "hazelcast/turbine-sidecar",
				},
				Hazelcast: &hazelcastv1alpha1.HazelcastReference{
					Cluster: &hazelcastv1alpha1.HazelcastRef{
						Name:      "hazelcast",
						Namespace: ns,
					},
				},
				Pods: &hazelcastv1alpha1.PodConfiguration{
					AppPortName: pointer.StringPtr("app-http"),
				},
			},
		}
	}

	PingDeployment = func(ns string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ping-service",
				Labels: map[string]string{
					"app":  "ping-service",
					"test": "true",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ping-service",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                        "ping-service",
							"turbine.hazelcast.com/name": "ping-pong",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:            "ping-service",
								Image:           "hakanmemisoglu/ping:0.1.0",
								ImagePullPolicy: "Always",
								Ports: []v1.ContainerPort{
									{
										Name:          "app-http",
										ContainerPort: 3000,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	PingService = func(ns string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ping-service",
				Namespace: ns,
				Labels: map[string]string{
					"test": "true",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": "ping-service",
				},
				Type: "LoadBalancer",
				Ports: []v1.ServicePort{
					{
						Port:     3000,
						Protocol: "TCP",
					},
				},
			},
		}
	}

	PongDeployment = func(ns string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pong-service",
				Labels: map[string]string{
					"app":  "pong-service",
					"test": "true",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "pong-service",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app":                        "pong-service",
							"turbine.hazelcast.com/name": "ping-pong",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:            "pong-service",
								Image:           "hakanmemisoglu/pong:0.1.0",
								ImagePullPolicy: "Always",
								Ports: []v1.ContainerPort{
									{
										Name:          "app-http",
										ContainerPort: 3001,
									},
								},
							},
						},
					},
				},
			},
		}
	}

	PongService = func(ns string) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pong-service",
				Namespace: ns,
				Labels: map[string]string{
					"test": "true",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": "pong-service",
				},
				Type: "LoadBalancer",
				Ports: []v1.ServicePort{
					{
						Port:     3001,
						Protocol: "TCP",
					},
				},
			},
		}
	}
)
