package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// HotBackupStatus defines the observed state of HotBackup
type HotBackupStatus struct {
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
	Schedule string `json:"schedule,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HotBackup is the Schema for the hot backup API
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
