package v1beta1

type Dependency struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
}

type Phase string

type BundleKind string

const (
	BundleKindHelm      BundleKind = "helm"
	BundleKindKustomize BundleKind = "kustomize"
	BundleKindUnknown   BundleKind = "unknown"
)

const (
	PhaseNone      Phase = "None" // No phase specified, plugin is not installed or removed
	PhaseInstalled Phase = "Installed"
	PhaseFailed    Phase = "Failed"
)

type GitSource struct {
	Revision string `json:"revision,omitempty"`
	Path     string `json:"path,omitempty"`
}

type S3Source struct {
	Bucket string `json:"bucket,omitempty"`
	Path   string `json:"path,omitempty"`
}

type HelmSource struct {
	Chart   string `json:"chart,omitempty"`
	Version string `json:"version,omitempty"`
}

type LocalFileSource struct {
	Path string `json:"path,omitempty"`
}

type HttpSource struct {
	Path string `json:"path,omitempty"` // path in the compressed file
}
