package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CronHotBackupSpec defines the desired state of CronHotBackup
type CronHotBackupSpec struct {
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
	// +kubebuilder:validation:MinLength:=1
	Schedule string `json:"schedule"`

	// Specifies the hot backup that will be created when executing a CronHotBackup.
	HotBackupTemplate HotBackupTemplateSpec `json:"hotBackupTemplate"`

	// The number of successful finished hot backups to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +kubebuilder:default:=5
	// +optional
	SuccessfulHotBackupsHistoryLimit *int32 `json:"successfulHotBackupsHistoryLimit,omitempty"`

	// The number of failed finished hot backups to retain.
	// This is a pointer to distinguish between explicit zero and not specified.
	// +kubebuilder:default:=3
	// +optional
	FailedHotBackupsHistoryLimit *int32 `json:"failedHotBackupsHistoryLimit,omitempty"`
}

type HotBackupTemplateSpec struct {
	// Standard object's metadata of the hot backups created from this template.
	// +optional
	// +kubebuilder:validation:XPreserveUnknownFields
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the hot backup.
	// +optional
	Spec HotBackupSpec `json:"spec,omitempty"`
}

// CronHotBackupStatus defines the observed state of CronHotBackup
type CronHotBackupStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=chb
// CronHotBackup is the Schema for the cronhotbackups API
type CronHotBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronHotBackupSpec   `json:"spec"`
	Status CronHotBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CronHotBackupList contains a list of CronHotBackup
type CronHotBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronHotBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronHotBackup{}, &CronHotBackupList{})
}
