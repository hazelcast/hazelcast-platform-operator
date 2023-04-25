package v1alpha1

import "sigs.k8s.io/controller-runtime/pkg/client"

// +k8s:deepcopy-gen=false
type CRLister interface {
	GetItems() []client.Object
}

func GetKind(obj client.Object) string {
	return obj.GetObjectKind().GroupVersionKind().Kind
}

// +k8s:deepcopy-gen=false
type DataStructure interface {
	GetDSName() string
	GetHZResourceName() string
	GetStatus() *DataStructureStatus
	GetSpec() (string, error)
	SetSpec(string) error
	ValidateSpecCurrent(h *Hazelcast) error
	ValidateSpecUpdate() error
}

type DataStructureSpec struct {
	// Name of the data structure config to be created. If empty, CR name will be used.
	// It cannot be updated after the config is created successfully.
	// +optional
	Name string `json:"name,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// Number of synchronous backups.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=1
	// +optional
	BackupCount *int32 `json:"backupCount,omitempty"`

	// Number of asynchronous backups.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=0
	// +optional
	AsyncBackupCount int32 `json:"asyncBackupCount"`
}

type DataStructureStatus struct {
	// State of the data structure
	// +optional
	State DataStructureConfigState `json:"state,omitempty"`
	// Message explaining the current state
	// +optional
	Message string `json:"message,omitempty"`
	// Holds status of data structure for each Hazelcast member
	// +optional
	MemberStatuses map[string]DataStructureConfigState `json:"memberStatuses,omitempty"`
}

// +kubebuilder:validation:Enum=Success;Failed;Pending;Persisting;Terminating
type DataStructureConfigState string

const (
	// Data structure is not successfully applied.
	DataStructureFailed DataStructureConfigState = "Failed"
	// Data structure configuration is applied successfully.
	DataStructureSuccess DataStructureConfigState = "Success"
	// Data structure configuration is being applied
	DataStructurePending DataStructureConfigState = "Pending"
	// The config is added into all members but waiting for the config to be persisted into ConfigMap
	DataStructurePersisting DataStructureConfigState = "Persisting"
	// Data structure is marked to be deleted,
	DataStructureTerminating DataStructureConfigState = "Terminating"
)
