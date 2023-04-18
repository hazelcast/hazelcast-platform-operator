package v1alpha1

import (
	"github.com/hazelcast/hazelcast-platform-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this Memcached to the Hub version (vbeta1).
func (src *Hazelcast) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.Hazelcast)
	dst.Spec.LicenseKeySecretName = src.Spec.LicenseKeySecret
	dst.ObjectMeta = src.ObjectMeta
	return nil
}

// ConvertFrom converts from the Hub version (vbeta1) to this version.
func (dst *Hazelcast) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.Hazelcast)
	dst.Spec.LicenseKeySecret = src.Spec.LicenseKeySecretName
	dst.ObjectMeta = src.ObjectMeta
	return nil
}
