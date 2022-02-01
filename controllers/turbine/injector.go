package turbine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type Injector struct {
	client    client.Client
	decoder   *admission.Decoder
	namespace string
}

func New(cli client.Client, ns string) *Injector {
	return &Injector{client: cli, namespace: ns}
}

// +kubebuilder:webhook:path=/inject-turbine,mutating=true,sideEffects="None",failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,admissionReviewVersions=v1,name=inject-turbine.hazelcast.com

func (i *Injector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &v1.Pod{}
	if err := i.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	name, namespace := inferMetadata(req, pod)
	if namespace != i.namespace {
		return admission.Allowed(
			fmt.Sprintf(
				"Pod {name: %s, namespace: %s} is not in namespace: %s",
				name,
				namespace,
				i.namespace,
			))
	}

	if !hasAnnotated(pod) {
		return admission.Allowed(
			fmt.Sprintf(
				"Pod {name: %s, namespace: %s, annotations: %v} is not annotated",
				name,
				namespace,
				pod.Annotations,
			))
	}

	if injected(pod) {
		return admission.Allowed(
			fmt.Sprintf(
				"Pod  {name: %s, namespace: %s} is already injected with turbine sidecar",
				name,
				namespace,
			))
	}

	pod = injectTurbineSidecar(pod)

	if result, err := json.Marshal(pod); err == nil {
		return admission.PatchResponseFromRaw(req.Object.Raw, result)
	} else {
		return admission.Errored(http.StatusInternalServerError, err)
	}
}

func injectTurbineSidecar(pod *v1.Pod) *v1.Pod {
	sidecar := v1.Container{
		Name:  "turbine-sidecar",
		Image: "hazelcast/turbine-sidecar",
		Env: []v1.EnvVar{
			{
				Name: "TURBINE_POD_IP",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		},
	}

	port := getAppPort(pod)
	if port != "" {
		sidecar.Env = append(sidecar.Env, v1.EnvVar{
			Name:  "APP_HTTP_PORT",
			Value: port,
		})
	}

	cm := getConfigMapName(pod)
	if cm != "" {
		sidecar.EnvFrom = append(sidecar.EnvFrom, envFromConfigmap(cm))
	}

	pod.Spec.Containers = append(pod.Spec.Containers, sidecar)

	return pod
}

func (i *Injector) InjectDecoder(d *admission.Decoder) error {
	i.decoder = d
	return nil
}

func injected(p *v1.Pod) bool {
	for _, c := range p.Spec.Containers {
		if c.Name == "turbine-sidecar" && c.Image == "hazelcast/turbine-sidecar" {
			return true
		}
	}
	return false
}

func hasAnnotated(p *v1.Pod) bool {
	val, ok := p.Annotations["turbine.hazelcast.com/enabled"]
	return ok && val == "true"
}

func inferMetadata(req admission.Request, pod *v1.Pod) (name string, namespace string) {
	if pod.Name != "" {
		name = pod.Name
	} else if req.Name != "" {
		name = req.Name
	} else {
		name = pod.GenerateName + "${RANDOM}"
	}

	if req.Namespace != "" {
		namespace = req.Namespace
	} else {
		namespace = pod.Namespace
	}

	return
}

func envFromConfigmap(name string) v1.EnvFromSource {
	return v1.EnvFromSource{
		ConfigMapRef: &v1.ConfigMapEnvSource{
			LocalObjectReference: v1.LocalObjectReference{
				Name: name,
			},
		},
	}
}

func getConfigMapName(p *v1.Pod) string {
	return p.Annotations["turbine.hazelcast.com/configmap"]
}

func getAppPort(p *v1.Pod) string {
	return p.Annotations["turbine.hazelcast.com/app-port"]
}
