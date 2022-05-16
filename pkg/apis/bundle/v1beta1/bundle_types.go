package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Status of the plugin"
// +kubebuilder:printcolumn:name="InstallNamespace",type="string",JSONPath=".status.installNamespace",description="Install Namespace of the plugin"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Version of the plugin"
// +kubebuilder:printcolumn:name="UpgradeTimestamp",type="date",JSONPath=".status.upgradeTimestamp",description="UpgradeTimestamp of the plugin"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp of the plugin"
type Bundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BundleSpec   `json:"spec,omitempty"`
	Status BundleStatus `json:"status,omitempty"`
}

type BundleSpec struct {
	// bundle kind (helm or kustomize)
	Kind BundleKind `json:"kind,omitempty"`

	URL string `json:"url,omitempty"`

	// from official helm repo
	// +optional
	Helm *HelmSource `json:"helm,omitempty"`

	// from s3 storage
	// +optional
	S3 *S3Source `json:"s3,omitempty"`

	// from git storage
	Git *GitSource `json:"git,omitempty"`

	// from curl or wget
	Http *HttpSource `json:"http,omitempty"`

	// plugin install namespace, same with metadata.namespace if empty.
	InstallNamespace string `json:"installNamespace,omitempty"`

	// dependends on other bundle
	Dependencies []Dependency `json:"dependencies,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// plugin values, helm values
	Values runtime.RawExtension `json:"values,omitempty"`
}

type BundleStatus struct {
	// Phase is the current state of the release
	Phase Phase `json:"phase,omitempty"`
	// Message is the message associated with the status
	Message string     `json:"message,omitempty"`
	Kind    BundleKind `json:"kind,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Values            runtime.RawExtension `json:"values,omitempty"`
	Version           string               `json:"version,omitempty"`
	CreationTimestamp metav1.Time          `json:"creationTimestamp,omitempty"`
	UpgradeTimestamp  metav1.Time          `json:"upgradeTimestamp,omitempty"`
	DeletionTimestamp *metav1.Time         `json:"deletionTimestamp,omitempty"`
	ManagedResource   []ManagedResource    `json:"managedResource,omitempty"`
	// Contains the rendered templates/NOTES.txt if available
	Notes string `json:"notes,omitempty"`
}

type ManagedResource struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	Error      string `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
type BundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Bundle `json:"items"`
}
