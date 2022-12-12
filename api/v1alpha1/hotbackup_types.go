package v1alpha1

import (
	"net/url"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:validation:Enum=Unknown;Pending;NotStarted;InProgress;Failure;Success
type HotBackupState string

const (
	HotBackupUnknown    HotBackupState = "Unknown"
	HotBackupPending    HotBackupState = "Pending"
	HotBackupNotStarted HotBackupState = "NotStarted"
	HotBackupInProgress HotBackupState = "InProgress"
	HotBackupFailure    HotBackupState = "Failure"
	HotBackupSuccess    HotBackupState = "Success"
)

func (s HotBackupState) IsFinished() bool {
	return s == HotBackupFailure || s == HotBackupSuccess
}

// IsRunning returns true if the HotBackup is scheduled to run or is running but not yet finished.
// Returns false if the HotBackup is not yet scheduled to run or finished it execution including the failure state.
func (s HotBackupState) IsRunning() bool {
	return s == HotBackupInProgress || s == HotBackupPending
}

// HotBackupStatus defines the observed state of HotBackup
type HotBackupStatus struct {
	// +optional
	State HotBackupState `json:"state,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	BackupUUIDs []string `json:"backupUUIDs,omitempty"`
}

func (hbs *HotBackupStatus) GetBucketURI() string {
	if len(hbs.BackupUUIDs) == 0 {
		return ""
	}
	u, err := url.ParseRequestURI(hbs.BackupUUIDs[0])
	if err != nil {
		return ""
	}

	values := u.Query()
	prefix := path.Dir(values.Get("prefix"))

	// if prefix ends with no /, add it
	if prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}

	values.Set("prefix", prefix)
	u.RawQuery, err = url.QueryUnescape(values.Encode())
	if err != nil {
		return ""
	}

	return u.String()
}

func (hbs *HotBackupStatus) GetBackupFolder() string {
	if len(hbs.BackupUUIDs) == 0 {
		return ""
	}
	return path.Dir(hbs.BackupUUIDs[0])
}

// HotBackupSpec defines the Spec of HotBackup
type HotBackupSpec struct {
	// HazelcastResourceName defines the name of the Hazelcast resource
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// URL of the bucket to download HotBackup folders.
	// +optional
	BucketURI string `json:"bucketURI,omitempty"`

	// Name of the secret with credentials for cloud providers.
	// +optional
	Secret string `json:"secret,omitempty"`
}

func (hbs *HotBackupSpec) IsExternal() bool {
	return hbs.BucketURI != "" && hbs.Secret != ""
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HotBackup is the Schema for the hot backup API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the HotBackup process"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.message",description="Message for the current HotBackup Config"
// +kubebuilder:resource:shortName=hb
type HotBackup struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	Spec HotBackupSpec `json:"spec"`
	// +optional
	Status HotBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HotBackupList contains a list of HotBackup
type HotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HotBackup `json:"items"`
}

func (hbl *HotBackupList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(hbl.Items))
	for _, item := range hbl.Items {
		l = append(l, client.Object(&item))
	}
	return l
}

func init() {
	SchemeBuilder.Register(&HotBackup{}, &HotBackupList{})
}
