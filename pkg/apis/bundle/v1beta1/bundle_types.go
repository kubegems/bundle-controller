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
	// Disabled indicates that the bundle should not be installed.
	Disabled bool `json:"disabled,omitempty"`

	// Kind bundle kind.
	Kind BundleKind `json:"kind,omitempty"`

	// URL is the URL of helm repository, git clone url, tarball url, s3 url, etc.
	URL string `json:"url,omitempty"`

	// Version is the version of helm chart, git revision, etc.
	Version string `json:"version,omitempty"`

	// Chart is the name of the chart to install.
	Chart string `json:"chart,omitempty"`

	// Path is the path in a tarball to the chart/kustomize.
	Path string `json:"path,omitempty"`

	// InstallNamespace is the namespace to install the bundle into.
	// If not specified, the bundle will be installed into the namespace of the bundle.
	InstallNamespace string `json:"installNamespace,omitempty"`

	// Dependencies is a list of bundles that this bundle depends on.
	// The bundle will be installed after all dependencies are exists.
	Dependencies []corev1.ObjectReference `json:"dependencies,omitempty"` // dependends on other bundle

	// Values is a nested map of helm values.
	// +kubebuilder:pruning:PreserveUnknownFields
	Values Values `json:"values,omitempty"`

	// ValuesFiles is a list of references to helm values files.
	// Ref can be a configmap or secret.
	// +kubebuilder:validation:Optional
	ValuesRef []ValuesRef `json:"valuesRef,omitempty"`
}

type ValuesRef struct {
	corev1.TypedLocalObjectReference `json:",inline"`

	// Optional set to true to ignore referense not found error
	Optional bool `json:"optional,omitempty"`
}

type BundleStatus struct {
	// Phase is the current state of the release
	Phase Phase `json:"phase,omitempty"`

	// Message is the message associated with the status
	// In helm, it's the notes contens.
	Message string `json:"message,omitempty"`

	// Values is a nested map of final helm values.
	// +kubebuilder:pruning:PreserveUnknownFields
	Values Values `json:"values,omitempty"`

	// Version is the version of the bundle.
	// In helm, Version is the version of the chart.
	Version string `json:"version,omitempty"`

	// Namespace is the namespace where the bundle is installed.
	Namespace string `json:"namespace,omitempty"`

	// CreationTimestamp is the first creation timestamp of the bundle.
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty"`

	// UpgradeTimestamp is the time when the bundle was last upgraded.
	UpgradeTimestamp metav1.Time `json:"upgradeTimestamp,omitempty"`

	// Resources is a list of resources created/managed by the bundle.
	Resources []corev1.ObjectReference `json:"resources,omitempty"`
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

type Phase string

// +kubebuilder:validation:Enum=helm;kustomize
type BundleKind string

const (
	BundleKindHelm      BundleKind = "helm"
	BundleKindKustomize BundleKind = "kustomize"
	BundleKindTemplate  BundleKind = "template"
)

const (
	PhaseDisabled  Phase = "Disabled"  // Bundle is disabled. the .spce.disbaled field is set to true or DeletionTimestamp is set.
	PhaseFailed    Phase = "Failed"    // Failed on install.
	PhaseInstalled Phase = "Installed" // Bundle is installed
)
