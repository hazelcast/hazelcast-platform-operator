package turbine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

const (
	injectedLabelKey   = "turbine.hazelcast.com/injected"
	injectedLabelValue = "true"

	skippedLabelKey   = "turbine.hazelcast.com/skipped-by"
	skippedLabelValue = "true"

	turbineNameLabelKey = "turbine.hazelcast.com/name"

	appPortAnnotationKey        = "turbine.hazelcast.com/app-port"
	clusterAddressAnnotationKey = "turbine.hazelcast.com/cluster-address"

	envPodIp          = "TURBINE_POD_IP"
	envAppHttpPort    = "APP_HTTP_PORT"
	envClusterAddress = "CLUSTER_ADDRESS"
)

type Injector struct {
	client    client.Client
	logger    logr.Logger
	decoder   *admission.Decoder
	namespace string
}

func New(cli client.Client, logger logr.Logger, ns string) *Injector {
	return &Injector{client: cli, logger: logger, namespace: ns}
}

func (i *Injector) InjectDecoder(d *admission.Decoder) error {
	i.decoder = d
	return nil
}

// +kubebuilder:webhook:path=/inject-turbine,mutating=true,sideEffects="None",failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,admissionReviewVersions=v1,name=inject-turbine.hazelcast.com

func (i *Injector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &v1.Pod{}
	if err := i.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	name, namespace := inferMetadata(req, pod)
	logger := i.logger.WithValues("podName", name, "podNamespace", namespace)

	turbine, err := i.getTurbineConfig(ctx, pod)
	if err != nil {
		logger.Error(err, "failed to get Turbine CR")
		return admission.Errored(http.StatusBadRequest, err)
	}

	pod = i.injectSidecar(ctx, pod, turbine)

	if result, err := json.Marshal(pod); err == nil {
		logger.Info("pod is patched successfully")
		return admission.PatchResponseFromRaw(req.Object.Raw, result)
	} else {
		logger.Error(err, "unable to marshal result pod object")
		return admission.Errored(http.StatusInternalServerError, err)
	}
}

func (i *Injector) injectSidecar(ctx context.Context, pod *v1.Pod, turbine *hazelcastv1alpha1.Turbine) *v1.Pod {
	p := pod.DeepCopy()
	sidecar := v1.Container{
		Name:  turbine.Spec.Sidecar.Name,
		Image: fmt.Sprintf("%s:%s", turbine.Spec.Sidecar.Repository, turbine.Spec.Sidecar.Version),
		Env: []v1.EnvVar{
			{
				Name: envPodIp,
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		},
	}
	missing := make([]string, 0)
	if port := getAppPort(p, turbine); port != "" {
		sidecar.Env = append(sidecar.Env, v1.EnvVar{
			Name:  envAppHttpPort,
			Value: port,
		})
	} else {
		missing = append(missing, envAppHttpPort)
	}

	if addr := i.getClusterAddress(ctx, p, turbine); addr != "" {
		sidecar.Env = append(sidecar.Env, v1.EnvVar{
			Name:  envClusterAddress,
			Value: addr,
		})
	} else {
		missing = append(missing, envClusterAddress)
	}

	if len(missing) > 0 {
		addSkippedLabel(pod)
	} else {
		pod.Spec.Containers = append(pod.Spec.Containers, sidecar)
		addInjectedLabel(pod)
		removeSkippedLabel(pod)
	}
	return pod
}

func (i *Injector) getTurbineConfig(ctx context.Context, p *v1.Pod) (*hazelcastv1alpha1.Turbine, error) {
	crName := p.Labels[turbineNameLabelKey]
	if crName == "" {
		return nil, errors.New(turbineNameLabelKey + " value is empty")
	}
	cr := &hazelcastv1alpha1.Turbine{}
	err := i.client.Get(ctx, types.NamespacedName{Name: crName}, cr)
	return cr, err
}

func (i *Injector) getClusterAddress(ctx context.Context, p *v1.Pod, turbine *hazelcastv1alpha1.Turbine) string {
	// Annotation takes precedence
	if addr, ok := p.Annotations[clusterAddressAnnotationKey]; ok && addr != "" {
		return addr
	}

	// If it fails, fallback to Turbine CR
	hzConf, addr := turbine.Spec.Hazelcast, ""
	if hzConf != nil && hzConf.ClusterAddress != nil {
		addr = *hzConf.ClusterAddress
	}
	if hzConf != nil && hzConf.Cluster != nil {
		addr = i.getClusterAddressFromCR(ctx, hzConf.Cluster.Name, hzConf.Cluster.Namespace)
	}
	return addr
}

func (i *Injector) getClusterAddressFromCR(ctx context.Context, name string, namespace string) string {
	svc := &v1.Service{}
	err := i.client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, svc)
	if err != nil {
		i.logger.Error(err, "failed to get Hazelcast service")
		return ""
	}
	return fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, svc.Spec.Ports[0].Port)
}

func getAppPort(p *v1.Pod, turbine *hazelcastv1alpha1.Turbine) string {
	// Annotation takes precedence
	if port, ok := p.Annotations[appPortAnnotationKey]; ok && port != "" {
		return port
	}

	// If annotation is not there, fallback to Turbine CR
	podsConf, portName := turbine.Spec.Pods, ""
	if podsConf != nil && podsConf.AppPortName != nil {
		portName = *podsConf.AppPortName
	}
	if portName == "" {
		return ""
	}
	for i := range p.Spec.Containers {
		for j := range p.Spec.Containers[i].Ports {
			if p.Spec.Containers[i].Ports[j].Name == portName {
				return strconv.FormatInt(int64(p.Spec.Containers[i].Ports[j].ContainerPort), 10)
			}
		}
	}
	return ""
}

func addInjectedLabel(p *v1.Pod) *v1.Pod {
	p.Labels[injectedLabelKey] = injectedLabelValue
	return p
}

func addSkippedLabel(p *v1.Pod) *v1.Pod {
	p.Labels[skippedLabelKey] = skippedLabelValue
	return p
}

func removeSkippedLabel(p *v1.Pod) *v1.Pod {
	delete(p.Labels, skippedLabelKey)
	return p
}

func inferMetadata(req admission.Request, pod *v1.Pod) (name string, namespace string) {
	if pod.Name != "" {
		name = pod.Name
	} else if req.Name != "" {
		name = req.Name
	} else {
		name = pod.GenerateName + "<RANDOM>"
	}

	if req.Namespace != "" {
		namespace = req.Namespace
	} else {
		namespace = pod.Namespace
	}

	return
}
