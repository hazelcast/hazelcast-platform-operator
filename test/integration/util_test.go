package integration

import (
	"context"
	"fmt"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	v1 "k8s.io/api/core/v1"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func assertDoesNotExist(name types.NamespacedName, obj client.Object) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err == nil {
			return false
		}
		return errors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())
}

func assertExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		return k8sClient.Get(context.Background(), name, obj)
	}, timeout, interval).Should(Succeed())
}

func assertExistsAndBeAsExpected[o client.Object](name types.NamespacedName, obj o, predicate func(o) bool) {
	Eventually(func() bool {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
			return false
		}
		return predicate(obj)
	}, timeout, interval).Should(BeTrue())
}

func deleteIfExists(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return k8sClient.Delete(context.Background(), obj)
	}, timeout, interval).Should(Succeed())
}

func deleteResource(name types.NamespacedName, obj client.Object) {
	Eventually(func() error {
		err := k8sClient.Get(context.Background(), name, obj)
		if err != nil {
			return err
		}
		return k8sClient.Delete(context.Background(), obj)
	}, timeout, interval).Should(Succeed())
}

func lookupKey(cr metav1.Object) types.NamespacedName {
	return types.NamespacedName{
		Name:      cr.GetName(),
		Namespace: cr.GetNamespace(),
	}
}

func getStatefulSet(cr metav1.Object) *appsv1.StatefulSet {
	sts := &appsv1.StatefulSet{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey(cr), sts)
	}, timeout, interval).Should(Succeed())

	return sts
}

func getSecret(cr metav1.Object) *v1.Secret {
	s := &v1.Secret{}
	Eventually(func() error {
		return k8sClient.Get(context.Background(), lookupKey(cr), s)
	}, timeout, interval).Should(Succeed())

	return s
}

func randomObjectMeta(ns string, annotations ...string) metav1.ObjectMeta {
	var annotationMap map[string]string

	if l := len(annotations); l > 0 {
		annotationMap = make(map[string]string)
		for i, str := range annotations {
			if i%2 == 0 {
				annotationMap[str] = ""
			} else {
				annotationMap[annotations[i-1]] = str
			}
		}
	}

	return metav1.ObjectMeta{
		Name:        fmt.Sprintf("%s", uuid.NewUUID()[0:5]),
		Namespace:   ns,
		Annotations: annotationMap,
	}
}

func defaultHazelcastSpecValues() *test.HazelcastSpecValues {
	licenseKey := ""
	repository := n.HazelcastRepo

	if ee {
		licenseKey = n.LicenseKeySecret
		repository = n.HazelcastEERepo
	}

	return &test.HazelcastSpecValues{
		ClusterSize:     n.DefaultClusterSize,
		Repository:      repository,
		Version:         n.HazelcastVersion,
		LicenseKey:      licenseKey,
		ImagePullPolicy: n.HazelcastImagePullPolicy,
	}
}

func defaultMcSpecValues() *test.MCSpecValues {
	return &test.MCSpecValues{
		Repository:      n.MCRepo,
		Version:         n.MCVersion,
		LicenseKey:      n.LicenseKeySecret,
		ImagePullPolicy: n.MCImagePullPolicy,
	}
}

// noinspection ALL
const (
	exampleCert = `-----BEGIN CERTIFICATE-----
MIIDBTCCAe2gAwIBAgIUIAQAd7v+j7HF1ReAvOmcRnKvhyowDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHZXhhbXBsZTAeFw0yMzAzMzAyMTM0MTVaFw0zMzAzMjcy
MTM0MTVaMBIxEDAOBgNVBAMMB2V4YW1wbGUwggEiMA0GCSqGSIb3DQEBAQUAA4IB
DwAwggEKAoIBAQClcwDpeKR8GPHSl43kNoARnZd0katAn1dLJA4xaumWmO6WTGMZ
CO5GkTA+4cfgLceSu/eRX0YKU8OdFjolQ3gB/qV/XEPkmrDet/fki8kwMxiDjxCA
TJ3BkOLwD1+kKjG/JYWGeSULa8osCQJoNttuY4Ep5cpZ1spLuJei0bItPZc4LRoq
pbblww1csIuz40xa6zM29nXwr/yVSk6gZLB0sqXlE1pLRwmm5yJVqwrb7yED63tp
5+R7E5Tths6NsvOuFlCfrLLLI8fjm7HECdnBlNVpfeSPP9gpmIGj8nqPSTelFVCe
VokegdMsiLXNmf/jAA5WLgF7nkvVPdw+bx1rAgMBAAGjUzBRMB0GA1UdDgQWBBQc
Jr/o86paLs6g7s02tfx27ALntTAfBgNVHSMEGDAWgBQcJr/o86paLs6g7s02tfx2
7ALntTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAEJZUFxde7
E2haX+zHTtFNe1reAtHiHL0oufJM4j77qhnbqBu1HzyDMekaNop18LFeI4wokINm
dhI6uWkog4r2oM0PGWys7fGSjxTdjf4imnsElbTDhVoCaUKpnPaP/TwcfHdZDdTB
dFddyuGVctC8+nGaHbQMT2IrRd7D0pOuSj5fMvgZmbUURSHFE1tzdjnH+uAAg+2+
FRxolQRHo8OvaWrProW8XMUEX5RD6fhtw/8OB3l66lKDy4fTSfyT3nulcmPsHUtS
R95Q5v7uTw/6roHb/By1jaQXtVkJ2WeM3wPO0IfWu02vFjjxpU4301Z19d7psF3t
+SnElJmXR3k5
-----END CERTIFICATE-----
	`

	exampleKey = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQClcwDpeKR8GPHS
l43kNoARnZd0katAn1dLJA4xaumWmO6WTGMZCO5GkTA+4cfgLceSu/eRX0YKU8Od
FjolQ3gB/qV/XEPkmrDet/fki8kwMxiDjxCATJ3BkOLwD1+kKjG/JYWGeSULa8os
CQJoNttuY4Ep5cpZ1spLuJei0bItPZc4LRoqpbblww1csIuz40xa6zM29nXwr/yV
Sk6gZLB0sqXlE1pLRwmm5yJVqwrb7yED63tp5+R7E5Tths6NsvOuFlCfrLLLI8fj
m7HECdnBlNVpfeSPP9gpmIGj8nqPSTelFVCeVokegdMsiLXNmf/jAA5WLgF7nkvV
Pdw+bx1rAgMBAAECggEAKvvSYFW+Ehme9fH25LP+GNWDDD9uKQdctAJlh5Q5pK0N
y1GEK3RlB0NgL+4Tshviri4Udxm0BinV9+FW8Ohy7L2+PHT5lJJV4j8UcbWZauLT
exZ3mIWPNMNSGkE8PVfS/dCfPJ0LsUhrSX57uByMbMUAQSTYqfeCLiMCjkQBkPv0
QwZ0gGyDJh53OgpdA4HV2PNvQ7fAlb8MiT16wKDoh/dnm19L4jiAZhK1cK+IWX6L
3e5mCR4DNUF+JIXTfMgOKkf39/9Gb84svFbjw6Txoog8lxTBmm4ldElst1w5yQ2J
sTB8VOw7Yo+ime38qy95U/ySYAgISoXmUldm5B9vZQKBgQDmvTV4VslyAy5tVjIL
LGQYUdRbsrl2WP4CrB9ja2F2yhJ59gtK8FuKbhMuTqVWvoKyq0C97HRzZoJoPml5
ArcRlFv4jWPl81lVHvnS25gtBRA7TzDfovq35gZ2itOFJzfyqnmNWpGBt5h/HxWC
040pTZ8ZAmAUoLHYxgZ7oS4gJwKBgQC3j/QVWKUtp0d+8op0Uo8/inEiNcaeyYkX
O6Te850gBJ0JH2TuAaQ6OTWwyINYo6VhMNOjDr6i5WP6zc1iIigUnUU+o7uoDsIt
oK5J1OUY4uPk93dXUGT5A9wfZi2bwPtb9QUnKUTRSvIAVaQdgtrgG+jMLnU4F2aN
3SkABOFfHQKBgQDXjPQxiim/95bcj0RKydpsGa2fSDQXigUpK/BaqQqwtQ9TnfVo
uWdax3/lp5Svl2NzU6Y0hns2/xFeHsfbQx0QMB9G75beT1opubk6MOhVTkCel1kZ
4iADwcBR51i4MC4E5RqOYYhCvOeaAcjPoZ9icV/qNhzZyFC8KCoQPj9fywKBgBM4
+PePS+TnAp6xqXwa9TNTPRu3A/C27CtJrK9IVaj3srY02m3uMBOE0DGOHesXYAc4
hMErlx0Z5olqKdrf9tCJ06mGne0wdncuv3Gt4LvlbrYYkB/NpHVLSS7klVwdLnVn
yD1cnf9I2OTeEwygGmmjopJXPyE7mhq7EUMWP7+lAoGBAMJZ3sWM5vFKIbIWHITB
eI9mvE13AoKkYqWeX9bdxlYefScfKhuweFMhVbm9+x+317iTQcVjueC2swHvxkHC
fFN8odcHpU+Fn5G00adcVcwqKoWx3RJPKUrs3GHiKKZhnZNw0niNxONm54k3zDrO
psSqtGKFs43q0BlH1z1zjcN+
-----END PRIVATE KEY-----
	`
)
