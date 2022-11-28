package v1alpha1

type DataStructureSpec struct {
	// Name of the data structure config to be created. If empty, CR name will be used.
	// It cannot be updated after the config is created successfully.
	// +optional
	Name string `json:"name,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource.
	// +kubebuilder:validation:MinLength:=1
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Number of synchronous backups.
	// +kubebuilder:default:=1
	// +optional
	BackupCount *int32 `json:"backupCount,omitempty"`

	// Number of asynchronous backups.
	// +kubebuilder:default:=0
	// +optional
	AsyncBackupCount *int32 `json:"asyncBackupCount,omitempty"`
}

type DataStructureStatus struct {
	State          DataStructureConfigState            `json:"state,omitempty"`
	Message        string                              `json:"message,omitempty"`
	MemberStatuses map[string]DataStructureConfigState `json:"memberStatuses,omitempty"`
}

// +kubebuilder:validation:Enum=Success;Failed;Pending;Persisting;Terminating
type DataStructureConfigState string

const (
	DataStructureFailed  DataStructureConfigState = "Failed"
	DataStructureSuccess DataStructureConfigState = "Success"
	DataStructurePending DataStructureConfigState = "Pending"
	// The config is added into all members but waiting for the config to be persisten into ConfigMap
	DataStructurePersisting  DataStructureConfigState = "Persisting"
	DataStructureTerminating DataStructureConfigState = "Terminating"
)
