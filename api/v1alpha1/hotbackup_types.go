package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	return !s.IsFinished() && s != ""
}

// HotBackupStatus defines the observed state of HotBackup
type HotBackupStatus struct {
	State HotBackupState `json:"state"`
}

// HotBackupSpec defines the Spec of HotBackup
type HotBackupSpec struct {
	// HazelcastResourceName defines the name of the Hazelcast resource
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Schedule contains a crontab-like expression that defines the schedule in which HotBackup will be started.
	// If the Schedule is empty the HotBackup will start only once when applied.
	// ---
	// Several pre-defined schedules in place of a cron expression can be used.
	//	Entry                  | Description                                | Equivalent To
	//	-----                  | -----------                                | -------------
	//	@yearly (or @annually) | Run once a year, midnight, Jan. 1st        | 0 0 1 1 *
	//	@monthly               | Run once a month, midnight, first of month | 0 0 1 * *
	//	@weekly                | Run once a week, midnight between Sat/Sun  | 0 0 * * 0
	//	@daily (or @midnight)  | Run once a day, midnight                   | 0 0 * * *
	//	@hourly                | Run once an hour, beginning of hour        | 0 * * * *
	// +optional
	Schedule string `json:"schedule"`

	// URL of the bucket to download HotBackup folders.
	// +optional
	BucketURL string `json:"bucket"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HotBackup is the Schema for the hot backup API
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="Current state of the HotBackup process"
type HotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Status HotBackupStatus `json:"status,omitempty"`
	Spec   HotBackupSpec   `json:"spec"`
}

//+kubebuilder:object:root=true

// HotBackupList contains a list of HotBackup
type HotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HotBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HotBackup{}, &HotBackupList{})
}
