package bundle

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

type PluginInstaller interface {
	Apply(ctx context.Context, plugin *Plugin) error
	Remove(ctx context.Context, plugin *Plugin) error
}

type Plugin struct {
	Kind       bundlev1.BundleKind
	Name       string
	Version    string
	Namespace  string
	Source     DownloadSource
	DownloadTo string
	Values     map[string]interface{}
	Status     PluginStatus
}

type PluginStatus struct {
	Phase               bundlev1.Phase
	Message             string
	Managed             []bundlev1.ManagedResource
	CreationTimestamp   metav1.Time
	LastUpdateTimestamp metav1.Time
	DeletionTimestamp   *metav1.Time
}

func (s *Plugin) ToPluginStatus() bundlev1.BundleStatus {
	return bundlev1.BundleStatus{
		Phase:             s.Status.Phase,
		Message:           s.Status.Message,
		Values:            MarshalValues(s.Values),
		Version:           s.Version,
		CreationTimestamp: s.Status.CreationTimestamp,
		UpgradeTimestamp:  s.Status.LastUpdateTimestamp,
		DeletionTimestamp: s.Status.DeletionTimestamp,
		ManagedResource:   s.Status.Managed,
	}
}

func PluginFromPlugin(bundle *bundlev1.Bundle) *Plugin {
	pplugin := &Plugin{
		Name:   bundle.Name,
		Kind:   bundle.Spec.Kind,
		Values: UnmarshalValues(bundle.Spec.Values),
		Source: DownloadSource{
			Helm: bundle.Spec.Helm,
			Git:  bundle.Spec.Git,
			S3:   bundle.Spec.S3,
			Http: bundle.Spec.Http,
		},
		Namespace: func() string {
			if bundle.Spec.InstallNamespace == "" {
				return bundle.Namespace
			}
			return bundle.Spec.InstallNamespace
		}(),
		Status: PluginStatus{
			Phase:               bundle.Status.Phase,
			Managed:             bundle.Status.ManagedResource,
			Message:             bundle.Status.Message,
			CreationTimestamp:   bundle.CreationTimestamp,
			LastUpdateTimestamp: bundle.Status.UpgradeTimestamp,
			DeletionTimestamp:   bundle.DeletionTimestamp,
		},
	}
	if helm := bundle.Spec.Helm; helm != nil {
		pplugin.Version = helm.Version
		if pplugin.Kind == "" {
			pplugin.Kind = bundlev1.BundleKindHelm
		}
	}
	if git := bundle.Spec.Git; git != nil {
		pplugin.Version = git.Revision
	}
	return pplugin
}

func MarshalValues(vals map[string]interface{}) runtime.RawExtension {
	if vals == nil {
		return runtime.RawExtension{}
	}
	bytes, _ := json.Marshal(vals)
	return runtime.RawExtension{Raw: bytes}
}

func UnmarshalValues(val runtime.RawExtension) map[string]interface{} {
	if val.Raw == nil {
		return nil
	}
	var vals interface{}
	_ = yaml.Unmarshal(val.Raw, &vals)

	if kvs, ok := vals.(map[string]interface{}); ok {
		RemoveNulls(kvs)
		return kvs
	}
	if arr, ok := vals.([]interface{}); ok {
		// is format of --set K=V
		kvs := make(map[string]interface{}, len(arr))
		for _, kv := range arr {
			if kv, ok := kv.(map[string]interface{}); ok {
				for k, v := range kv {
					kvs[k] = v
				}
			}
		}
		RemoveNulls(kvs)
		return kvs
	}
	return nil
}

// https://github.com/helm/helm/blob/bed1a42a398b30a63a279d68cc7319ceb4618ec3/pkg/chartutil/coalesce.go#L37
// helm CoalesceValues cant handle nested null,like `{a: {b: null}}`, which want to be `{}`
func RemoveNulls(m map[string]interface{}) {
	for k, v := range m {
		if val, ok := v.(map[string]interface{}); ok {
			RemoveNulls(val)
			if len(val) == 0 {
				delete(m, k)
			}
			continue
		}
		if v == nil {
			delete(m, k)
			continue
		}
	}
}

type DelegateManager struct {
	Managers map[bundlev1.BundleKind]PluginInstaller
}

func NewDelegateManager(mgr ctrl.Manager) *DelegateManager {
	return &DelegateManager{
		Managers: map[bundlev1.BundleKind]PluginInstaller{
			bundlev1.BundleKindHelm:      NewHelmPlugin(mgr.GetConfig()),
			bundlev1.BundleKindKustomize: NewKustomizePlugin(mgr.GetClient()),
		},
	}
}

func (m *DelegateManager) Apply(ctx context.Context, plugin *Plugin) error {
	// we download it first
	if err := DownloadPlugin(ctx, plugin, ""); err != nil {
		return err
	}
	if manager, ok := m.Managers[plugin.Kind]; ok {
		return manager.Apply(ctx, plugin)
	}
	return fmt.Errorf("unspported kind %s", plugin.Kind)
}

func (m *DelegateManager) Remove(ctx context.Context, plugin *Plugin) error {
	if manager, ok := m.Managers[plugin.Kind]; ok {
		return manager.Apply(ctx, plugin)
	}
	return fmt.Errorf("unspported kind %s", plugin.Kind)
}
