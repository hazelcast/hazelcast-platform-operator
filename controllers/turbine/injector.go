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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	injectedLabelKey   = "turbine.hazelcast.com/injected"
	injectedLabelValue = "true"

	injectionEnabledLabelKey   = "turbine.hazelcast.com/enabled"
	injectionEnabledLabelValue = "true"

	configMapNameAnnotationKey = "turbine.hazelcast.com/configmap"
	appPortAnnotationKey       = "turbine.hazelcast.com/app-port"

	sidecarName  = "turbine-sidecar"
	sidecarImage = "hazelcast/turbine-sidecar"

	envPodIp       = "TURBINE_POD_IP"
	envAppHttpPort = "APP_HTTP_PORT"

	appHttpPortName = "app-http"
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

// +kubebuilder:webhook:path=/inject-turbine,mutating=true,sideEffects="None",failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,admissionReviewVersions=v1,name=inject-turbine.hazelcast.com

func (i *Injector) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &v1.Pod{}
	if err := i.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	name, namespace := inferMetadata(req, pod)
	logger := i.logger.WithValues("podName", name, "podNamespace", namespace)

	// Not needed since the object selection in webhook configuration, we are doing it anyway.
	if !hasInjectionEnabled(pod) {
		logger.Error(errors.New("webhook received pod with unexpected condition"), "unexpected pod label condition")
		return admission.Allowed(
			fmt.Sprintf(
				"pod {name: %s, namespace: %s} is not labeled",
				name,
				namespace,
			))
	}

	status := getInjectionStatus(pod)

	switch status {
	case LabeledWithSidecar:
		return admission.Allowed(
			fmt.Sprintf(
				"pod {name: %s, namespace: %s} is already injected with turbine sidecar",
				name,
				namespace,
			))

	case LabeledWithoutSidecar:
		logger.Error(errors.New("pod was labeled before but it was not injected"), "unexpected condition")
		pod = injectTurbineSidecar(pod)

	case NoLabelWithSidecar:
		logger.Error(errors.New("pod was injected by another entity"), "unexpected condition")
		pod = addInjectedLabel(pod)

	case NoLabelWithoutSidecar:
		pod = injectTurbineSidecar(pod)
	}

	if result, err := json.Marshal(pod); err == nil {
		logger.Info("pod is patched successfully")
		return admission.PatchResponseFromRaw(req.Object.Raw, result)
	} else {
		i.logger.Error(err, "unable to marshal result pod object")
		return admission.Errored(http.StatusInternalServerError, err)
	}
}

func (i *Injector) InjectDecoder(d *admission.Decoder) error {
	i.decoder = d
	return nil
}

func injectTurbineSidecar(pod *v1.Pod) *v1.Pod {
	sidecar := v1.Container{
		Name:  sidecarName,
		Image: sidecarImage,
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

	port := getAppPort(pod)
	if port != "" {
		sidecar.Env = append(sidecar.Env, v1.EnvVar{
			Name:  envAppHttpPort,
			Value: port,
		})
	}

	cm := getConfigMapName(pod)
	if cm != "" {
		sidecar.EnvFrom = append(sidecar.EnvFrom, envFromConfigmap(cm))
	}

	pod.Spec.Containers = append(pod.Spec.Containers, sidecar)

	pod = addInjectedLabel(pod)
	return pod
}

func addInjectedLabel(p *v1.Pod) *v1.Pod {
	p.Labels[injectedLabelKey] = injectedLabelValue
	return p
}

type injectionStatus int

const (
	LabeledWithSidecar = iota
	LabeledWithoutSidecar
	NoLabelWithSidecar
	NoLabelWithoutSidecar
)

func getInjectionStatus(p *v1.Pod) injectionStatus {
	sideCarExists := hasTurbineSidecar(p)
	labeled := isLabeledAsInjected(p)

	if sideCarExists && labeled {
		return LabeledWithSidecar
	} else if sideCarExists {
		return NoLabelWithSidecar
	} else if labeled {
		return LabeledWithoutSidecar
	} else {
		return NoLabelWithoutSidecar
	}
}

func hasTurbineSidecar(p *v1.Pod) bool {
	for _, c := range p.Spec.Containers {
		if c.Name == sidecarName && c.Image == sidecarImage {
			return true
		}
	}
	return false
}

func isLabeledAsInjected(p *v1.Pod) bool {
	val, ok := p.Labels[injectedLabelKey]
	return ok && val == injectedLabelValue
}

func hasInjectionEnabled(p *v1.Pod) bool {
	val, ok := p.Labels[injectionEnabledLabelKey]
	return ok && val == injectionEnabledLabelValue
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
	return p.Annotations[configMapNameAnnotationKey]
}

func getAppPort(p *v1.Pod) string {
	if port, ok := p.Annotations[appPortAnnotationKey]; ok && port != "" {
		return port
	}

	for i := range p.Spec.Containers {
		for j := range p.Spec.Containers[i].Ports {
			if p.Spec.Containers[i].Ports[j].Name == appHttpPortName {
				return strconv.FormatInt(int64(p.Spec.Containers[i].Ports[j].ContainerPort), 10)
			}
		}
	}

	return ""
}
