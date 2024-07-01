package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:validation:Enum=Failed;NotRunning;Starting;Running;Suspended;SuspendedExportingSnapshot;Completing;ExecutionFailed;Completed
type JetJobStatusPhase string

const (
	JetJobFailed                     JetJobStatusPhase = "Failed"
	JetJobNotRunning                 JetJobStatusPhase = "NotRunning"                 //0
	JetJobStarting                   JetJobStatusPhase = "Starting"                   //1
	JetJobRunning                    JetJobStatusPhase = "Running"                    //2
	JetJobSuspended                  JetJobStatusPhase = "Suspended"                  //3
	JetJobSuspendedExportingSnapshot JetJobStatusPhase = "SuspendedExportingSnapshot" //4
	JetJobCompleting                 JetJobStatusPhase = "Completing"                 //5
	JetJobExecutionFailed            JetJobStatusPhase = "ExecutionFailed"            //6
	JetJobCompleted                  JetJobStatusPhase = "Completed"                  //7
)

type JetJobState string

const (
	RunningJobState   JetJobState = "Running"
	SuspendedJobState JetJobState = "Suspended"
	CanceledJobState  JetJobState = "Canceled"
	RestartedJobState JetJobState = "Restarted"
)

// JetJobSpec defines the desired state of JetJob
type JetJobSpec struct {
	// Name of the JetJob to be created. If empty, CR name will be used.
	// It cannot be updated after the config is created successfully.
	// +optional
	Name string `json:"name,omitempty"`

	// HazelcastResourceName defines the name of the Hazelcast resource that this resource is
	// created for.
	// +kubebuilder:validation:MinLength:=1
	// +required
	HazelcastResourceName string `json:"hazelcastResourceName"`

	// State is used to manage the job state.
	// +kubebuilder:default:=Running
	// +kubebuilder:validation:Enum=Running;Suspended;Canceled;Restarted
	// +required
	State JetJobState `json:"state"`

	// JarName specify the name of the Jar to run that is present on the member.
	// +kubebuilder:validation:MinLength:=1
	// +required
	JarName string `json:"jarName"`

	// MainClass is the name of the main class that will be run on the submitted job.
	// +optional
	MainClass string `json:"mainClass,omitempty"`

	// InitialSnapshotResourceName specify the name of the JetJobSnapshot object from which
	// the JetJob is initialized.
	// +optional
	InitialSnapshotResourceName string `json:"initialSnapshotResourceName,omitempty"`

	// Parameters to be passed to Jet Job.
	// +optional
	Parameters []string `json:"parameters,omitempty"`

	// Configuration for downloading the file from remote.
	// +optional
	JetRemoteFileConfiguration `json:",inline"`
}

type JetRemoteFileConfiguration struct {
	// Bucket config from where the JAR files will be downloaded.
	// +optional
	BucketConfiguration *BucketConfiguration `json:"bucketConfig,omitempty"`

	// URL from where the file will be downloaded.
	// +optional
	RemoteURL string `json:"remoteURL,omitempty"`
}

// Returns true is eigher of bucketConfiguration or remoteURL are enabled
func (j *JetJobSpec) IsDownloadEnabled() bool {
	return j.IsBucketEnabled() || j.IsRemoteURLsEnabled()
}

// Returns true if bucketConfiguration is specified.
func (j *JetJobSpec) IsBucketEnabled() bool {
	return j != nil && j.JetRemoteFileConfiguration.BucketConfiguration != nil
}

// Returns true if remoteURL configuration is specified.
func (j *JetJobSpec) IsRemoteURLsEnabled() bool {
	return j != nil && j.JetRemoteFileConfiguration.RemoteURL != ""
}

// JetJobStatus defines the observed state of JetJob
type JetJobStatus struct {
	Id    int64             `json:"id"`
	Phase JetJobStatusPhase `json:"phase"`
	// +optional
	SubmissionTime *metav1.Time `json:"submissionTime,omitempty"`
	// +optional
	CompletionTime  *metav1.Time `json:"completionTime,omitempty"`
	FailureText     string       `json:"failureText,omitempty"`
	SuspensionCause string       `json:"suspensionCause,omitempty"`
}

func (jjs JetJobStatusPhase) IsRunning() bool {
	return jjs == JetJobRunning || jjs == JetJobStarting
}

func (jjs JetJobStatusPhase) IsFinished() bool {
	return jjs == JetJobExecutionFailed || jjs == JetJobCompleted
}

func (jjs JetJobStatusPhase) IsSuspended() bool {
	return jjs == JetJobSuspended || jjs == JetJobSuspendedExportingSnapshot
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Current state of the JetJob"
// +kubebuilder:printcolumn:name="Id",type="string",JSONPath=".status.id",description="ID of the JetJob"
// +kubebuilder:printcolumn:name="Hazelcast-Resource",type="string",priority=1,JSONPath=".spec.hazelcastResourceName",description="Name of the Hazelcast resource that this resource is created for"
// +kubebuilder:printcolumn:name="SubmissionTime",type="string",JSONPath=".status.submissionTime",description="Time when the JetJob was submitted"
// +kubebuilder:printcolumn:name="CompletionTime",type="string",JSONPath=".status.completionTime",description="Time when the JetJob was completed"
// +kubebuilder:resource:shortName=jj

// JetJob is the Schema for the jetjobs API
type JetJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JetJobSpec   `json:"spec,omitempty"`
	Status JetJobStatus `json:"status,omitempty"`
}

func (j *JetJob) JobName() string {
	if j.Spec.Name != "" {
		return j.Spec.Name
	}
	return j.Name
}

//+kubebuilder:object:root=true

// JetJobList contains a list of JetJob
type JetJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JetJob `json:"items"`
}

func (jjl *JetJobList) GetItems() []client.Object {
	l := make([]client.Object, 0, len(jjl.Items))
	for _, item := range jjl.Items {
		l = append(l, client.Object(&item))
	}
	return l
}

func init() {
	SchemeBuilder.Register(&JetJob{}, &JetJobList{})
}
