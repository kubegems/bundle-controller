package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase",description="Status of the bundle"
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".status.namespace",description="Install Namespace of the bundle"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version",description="Version of the bundle"
// +kubebuilder:printcolumn:name="UpgradeTimestamp",type="date",JSONPath=".status.upgradeTimestamp",description="UpgradeTimestamp of the bundle"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="CreationTimestamp of the bundle"
type Bundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BundleSpec   `json:"spec,omitempty"`
	Status BundleStatus `json:"status,omitempty"`
}

type BundleSpec struct {
	// bundle kind (helm or kustomize)
	Kind             BundleKind   `json:"kind,omitempty"`
	URL              string       `json:"url,omitempty"`
	Version          string       `json:"version,omitempty"`
	Chart            string       `json:"chart,omitempty"`
	Path             string       `json:"path,omitempty"`
	InstallNamespace string       `json:"installNamespace,omitempty"`
	Dependencies     []Dependency `json:"dependencies,omitempty"` // dependends on other bundle

	// +kubebuilder:pruning:PreserveUnknownFields
	Values    Values      `json:"values,omitempty"`
	ValuesRef []ValuesRef `json:"valuesRef,omitempty"`
}

type ValuesRef struct {
	corev1.TypedLocalObjectReference `json:",inline"`
	Optional                         bool `json:"optional,omitempty"`
}

type BundleStatus struct {
	// Phase is the current state of the release
	Phase Phase `json:"phase,omitempty"`
	// Message is the message associated with the status
	Message string `json:"message,omitempty"`
	// +kubebuilder:pruning:PreserveUnknownFields
	Values            Values            `json:"values,omitempty"`
	Version           string            `json:"version,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	CreationTimestamp metav1.Time       `json:"creationTimestamp,omitempty"`
	UpgradeTimestamp  metav1.Time       `json:"upgradeTimestamp,omitempty"`
	Resources         []ManagedResource `json:"resources,omitempty"`
}

type ManagedResource struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
}

//+kubebuilder:object:root=true
type BundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Bundle `json:"items"`
}
