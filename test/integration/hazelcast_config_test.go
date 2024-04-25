package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
)

var _ = Describe("Hazelcast Config Secret", func() {
	const namespace = "default"

	FetchConfigSecret := func(h *hazelcastv1alpha1.Hazelcast) *v1.Secret {
		sc := &v1.Secret{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: h.Name, Namespace: h.Namespace}, sc)).Should(Succeed())
		return sc
	}

	GetHzConfig := func(h *hazelcastv1alpha1.Hazelcast) map[string]interface{} {
		s := FetchConfigSecret(h)
		expectedMap := make(map[string]interface{})
		Expect(yaml.Unmarshal(s.Data["hazelcast.yaml"], expectedMap)).Should(Succeed())
		return expectedMap[n.HazelcastCustomConfigKey].(map[string]interface{})
	}

	BeforeEach(func() {
		if ee {
			By(fmt.Sprintf("creating license key secret '%s'", n.LicenseDataKey))
			licenseKeySecret := CreateLicenseKeySecret(n.LicenseKeySecret, namespace)
			assertExists(lookupKey(licenseKeySecret), licenseKeySecret)
		}
	})

	AfterEach(func() {
		DeleteAllOf(&hazelcastv1alpha1.Hazelcast{}, nil, namespace, map[string]string{})
	})

	Context("with custom configs", func() {
		It("should add new section to config", func() {
			customConfig := make(map[string]interface{})
			sc := make(map[string]interface{})
			sc["portable-version"] = 0
			sc["use-native-byte-order"] = false
			sc["byte-order"] = "BIG_ENDIAN"
			sc["check-class-def-errors"] = true
			customConfig["serialization"] = sc
			cm := &v1.ConfigMap{
				ObjectMeta: randomObjectMeta(namespace),
			}
			out, err := yaml.Marshal(customConfig)
			Expect(err).To(BeNil())
			cm.Data = make(map[string]string)
			cm.Data[n.HazelcastCustomConfigKey] = string(out)
			Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.CustomConfigCmName = cm.Name
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			assertHzStatusIsPending(hz)

			hzConfig := GetHzConfig(hz)
			Expect(hzConfig).Should(And(
				HaveKey("advanced-network"), HaveKey("cluster-name"), HaveKey("jet"), HaveKey("serialization")))
			expectedSer := hzConfig["serialization"].(map[string]interface{})
			Expect(expectedSer).To(Equal(sc))
		})

		It("should not override CR configs", func() {
			customConfig := make(map[string]interface{})
			ucdConf := make(map[string]interface{})
			ucdConf["enabled"] = true
			ucdConf["class-cache-mode"] = "ETERNAL"
			ucdConf["provider-mode"] = "LOCAL_AND_CACHED_CLASSES"
			ucdConf["blacklist-prefixes"] = "com.foo,com.bar"
			ucdConf["whitelist-prefixes"] = "com.bar.MyClass"
			ucdConf["provider-filter"] = "HAS_ATTRIBUTE:lite"
			customConfig["user-code-deployment"] = ucdConf
			cm := &v1.ConfigMap{
				ObjectMeta: randomObjectMeta(namespace),
			}
			out, err := yaml.Marshal(customConfig)
			Expect(err).To(BeNil())
			cm.Data = make(map[string]string)
			cm.Data[n.HazelcastCustomConfigKey] = string(out)
			Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())
			spec := test.HazelcastSpec(defaultHazelcastSpecValues(), ee)
			spec.UserCodeDeployment = &hazelcastv1alpha1.UserCodeDeploymentConfig{
				ClientEnabled: pointer.Bool(false),
			}
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       spec,
			}
			hz.Spec.CustomConfigCmName = cm.Name
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			assertHzStatusIsPending(hz)

			hzConfig := GetHzConfig(hz)
			Expect(hzConfig).Should(HaveKey("user-code-deployment"))
			Expect(hzConfig["user-code-deployment"]).Should(HaveKeyWithValue("enabled", false))
			Expect(hzConfig["user-code-deployment"]).Should(And(
				Not(HaveKey("class-cache-mode")), Not(HaveKey("provider-filter")), Not(HaveKey("provider-mode"))))
		})

		It("should not override advanced network config", func() {
			customConfig := make(map[string]interface{})
			anConf := make(map[string]interface{})
			anConf["enabled"] = false
			customConfig["advanced-network"] = anConf
			cm := &v1.ConfigMap{
				ObjectMeta: randomObjectMeta(namespace),
			}
			out, err := yaml.Marshal(customConfig)
			Expect(err).To(BeNil())
			cm.Data = make(map[string]string)
			cm.Data[n.HazelcastCustomConfigKey] = string(out)
			Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.CustomConfigCmName = cm.Name
			Expect(k8sClient.Create(context.Background(), hz)).Should(Succeed())
			assertHzStatusIsPending(hz)

			hzConfig := GetHzConfig(hz)
			Expect(hzConfig).Should(HaveKey("advanced-network"))
			Expect(hzConfig["advanced-network"]).Should(HaveKeyWithValue("enabled", true))
			Expect(hzConfig["advanced-network"]).Should(And(
				HaveKey("client-server-socket-endpoint-config"),
				HaveKey("member-server-socket-endpoint-config"),
				HaveKey("rest-server-socket-endpoint-config")))
		})

		It("should fail if the config map does not exist", func() {
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.CustomConfigCmName = "cm"
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("ConfigMap for Hazelcast custom configs not found")))
		})

		It("should fail if the config map does not contain the expected key", func() {
			cm := &v1.ConfigMap{
				ObjectMeta: randomObjectMeta(namespace),
			}
			cm.Data = make(map[string]string)
			Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.CustomConfigCmName = cm.Name
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("ConfigMap for Hazelcast custom configs must contain 'hazelcast' key")))
		})

		It("should fail if the value in the config map is not a valid yaml", func() {
			cm := &v1.ConfigMap{
				ObjectMeta: randomObjectMeta(namespace),
			}
			cm.Data = make(map[string]string)
			cm.Data[n.HazelcastCustomConfigKey] = "it: is: invalid: yaml"
			Expect(k8sClient.Create(context.Background(), cm)).Should(Succeed())
			hz := &hazelcastv1alpha1.Hazelcast{
				ObjectMeta: randomObjectMeta(namespace),
				Spec:       test.HazelcastSpec(defaultHazelcastSpecValues(), ee),
			}
			hz.Spec.CustomConfigCmName = cm.Name
			Expect(k8sClient.Create(context.Background(), hz)).
				Should(MatchError(ContainSubstring("ConfigMap for Hazelcast custom configs is not a valid yaml")))
		})
	})
})
