//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExposeExternallyConfiguration) DeepCopyInto(out *ExposeExternallyConfiguration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExposeExternallyConfiguration.
func (in *ExposeExternallyConfiguration) DeepCopy() *ExposeExternallyConfiguration {
	if in == nil {
		return nil
	}
	out := new(ExposeExternallyConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExternalConnectivityConfiguration) DeepCopyInto(out *ExternalConnectivityConfiguration) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExternalConnectivityConfiguration.
func (in *ExternalConnectivityConfiguration) DeepCopy() *ExternalConnectivityConfiguration {
	if in == nil {
		return nil
	}
	out := new(ExternalConnectivityConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Hazelcast) DeepCopyInto(out *Hazelcast) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Hazelcast.
func (in *Hazelcast) DeepCopy() *Hazelcast {
	if in == nil {
		return nil
	}
	out := new(Hazelcast)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Hazelcast) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastClusterConfig) DeepCopyInto(out *HazelcastClusterConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastClusterConfig.
func (in *HazelcastClusterConfig) DeepCopy() *HazelcastClusterConfig {
	if in == nil {
		return nil
	}
	out := new(HazelcastClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastClusterStatus) DeepCopyInto(out *HazelcastClusterStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastClusterStatus.
func (in *HazelcastClusterStatus) DeepCopy() *HazelcastClusterStatus {
	if in == nil {
		return nil
	}
	out := new(HazelcastClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastList) DeepCopyInto(out *HazelcastList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Hazelcast, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastList.
func (in *HazelcastList) DeepCopy() *HazelcastList {
	if in == nil {
		return nil
	}
	out := new(HazelcastList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HazelcastList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastMemberStatus) DeepCopyInto(out *HazelcastMemberStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastMemberStatus.
func (in *HazelcastMemberStatus) DeepCopy() *HazelcastMemberStatus {
	if in == nil {
		return nil
	}
	out := new(HazelcastMemberStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastPersistenceConfiguration) DeepCopyInto(out *HazelcastPersistenceConfiguration) {
	*out = *in
	in.Pvc.DeepCopyInto(&out.Pvc)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastPersistenceConfiguration.
func (in *HazelcastPersistenceConfiguration) DeepCopy() *HazelcastPersistenceConfiguration {
	if in == nil {
		return nil
	}
	out := new(HazelcastPersistenceConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastSpec) DeepCopyInto(out *HazelcastSpec) {
	*out = *in
	if in.ClusterSize != nil {
		in, out := &in.ClusterSize, &out.ClusterSize
		*out = new(int32)
		**out = **in
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]v1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.ExposeExternally != nil {
		in, out := &in.ExposeExternally, &out.ExposeExternally
		*out = new(ExposeExternallyConfiguration)
		**out = **in
	}
	if in.Scheduling != nil {
		in, out := &in.Scheduling, &out.Scheduling
		*out = new(SchedulingConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Persistence != nil {
		in, out := &in.Persistence, &out.Persistence
		*out = new(HazelcastPersistenceConfiguration)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastSpec.
func (in *HazelcastSpec) DeepCopy() *HazelcastSpec {
	if in == nil {
		return nil
	}
	out := new(HazelcastSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HazelcastStatus) DeepCopyInto(out *HazelcastStatus) {
	*out = *in
	out.Cluster = in.Cluster
	if in.Members != nil {
		in, out := &in.Members, &out.Members
		*out = make([]HazelcastMemberStatus, len(*in))
		copy(*out, *in)
	}
	if in.Restore != nil {
		in, out := &in.Restore, &out.Restore
		*out = new(RestoreStatus)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HazelcastStatus.
func (in *HazelcastStatus) DeepCopy() *HazelcastStatus {
	if in == nil {
		return nil
	}
	out := new(HazelcastStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HotBackup) DeepCopyInto(out *HotBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Status = in.Status
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HotBackup.
func (in *HotBackup) DeepCopy() *HotBackup {
	if in == nil {
		return nil
	}
	out := new(HotBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HotBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HotBackupList) DeepCopyInto(out *HotBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]HotBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HotBackupList.
func (in *HotBackupList) DeepCopy() *HotBackupList {
	if in == nil {
		return nil
	}
	out := new(HotBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *HotBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HotBackupSpec) DeepCopyInto(out *HotBackupSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HotBackupSpec.
func (in *HotBackupSpec) DeepCopy() *HotBackupSpec {
	if in == nil {
		return nil
	}
	out := new(HotBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HotBackupStatus) DeepCopyInto(out *HotBackupStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HotBackupStatus.
func (in *HotBackupStatus) DeepCopy() *HotBackupStatus {
	if in == nil {
		return nil
	}
	out := new(HotBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagementCenter) DeepCopyInto(out *ManagementCenter) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagementCenter.
func (in *ManagementCenter) DeepCopy() *ManagementCenter {
	if in == nil {
		return nil
	}
	out := new(ManagementCenter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ManagementCenter) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagementCenterList) DeepCopyInto(out *ManagementCenterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ManagementCenter, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagementCenterList.
func (in *ManagementCenterList) DeepCopy() *ManagementCenterList {
	if in == nil {
		return nil
	}
	out := new(ManagementCenterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ManagementCenterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagementCenterSpec) DeepCopyInto(out *ManagementCenterSpec) {
	*out = *in
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]v1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.HazelcastClusters != nil {
		in, out := &in.HazelcastClusters, &out.HazelcastClusters
		*out = make([]HazelcastClusterConfig, len(*in))
		copy(*out, *in)
	}
	if in.ExternalConnectivity != nil {
		in, out := &in.ExternalConnectivity, &out.ExternalConnectivity
		*out = new(ExternalConnectivityConfiguration)
		**out = **in
	}
	if in.Persistence != nil {
		in, out := &in.Persistence, &out.Persistence
		*out = new(PersistenceConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Scheduling != nil {
		in, out := &in.Scheduling, &out.Scheduling
		*out = new(SchedulingConfiguration)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagementCenterSpec.
func (in *ManagementCenterSpec) DeepCopy() *ManagementCenterSpec {
	if in == nil {
		return nil
	}
	out := new(ManagementCenterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagementCenterStatus) DeepCopyInto(out *ManagementCenterStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagementCenterStatus.
func (in *ManagementCenterStatus) DeepCopy() *ManagementCenterStatus {
	if in == nil {
		return nil
	}
	out := new(ManagementCenterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PersistenceConfiguration) DeepCopyInto(out *PersistenceConfiguration) {
	*out = *in
	if in.StorageClass != nil {
		in, out := &in.StorageClass, &out.StorageClass
		*out = new(string)
		**out = **in
	}
	if in.Size != nil {
		in, out := &in.Size, &out.Size
		x := (*in).DeepCopy()
		*out = &x
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PersistenceConfiguration.
func (in *PersistenceConfiguration) DeepCopy() *PersistenceConfiguration {
	if in == nil {
		return nil
	}
	out := new(PersistenceConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PersistencePvcConfiguration) DeepCopyInto(out *PersistencePvcConfiguration) {
	*out = *in
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
	if in.RequestStorage != nil {
		in, out := &in.RequestStorage, &out.RequestStorage
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PersistencePvcConfiguration.
func (in *PersistencePvcConfiguration) DeepCopy() *PersistencePvcConfiguration {
	if in == nil {
		return nil
	}
	out := new(PersistencePvcConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RestoreStatus) DeepCopyInto(out *RestoreStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RestoreStatus.
func (in *RestoreStatus) DeepCopy() *RestoreStatus {
	if in == nil {
		return nil
	}
	out := new(RestoreStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SchedulingConfiguration) DeepCopyInto(out *SchedulingConfiguration) {
	*out = *in
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.TopologySpreadConstraints != nil {
		in, out := &in.TopologySpreadConstraints, &out.TopologySpreadConstraints
		*out = make([]v1.TopologySpreadConstraint, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SchedulingConfiguration.
func (in *SchedulingConfiguration) DeepCopy() *SchedulingConfiguration {
	if in == nil {
		return nil
	}
	out := new(SchedulingConfiguration)
	in.DeepCopyInto(out)
	return out
}
