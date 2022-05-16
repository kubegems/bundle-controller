package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/helm"
	"sigs.k8s.io/yaml"
)

type HelmApplier struct {
	Helm *helm.Helm
}

func NewHelm(config *rest.Config) *HelmApplier {
	return &HelmApplier{Helm: &helm.Helm{Config: config}}
}

func (r *HelmApplier) Apply(ctx context.Context, bundle *bundlev1.Bundle, into string) error {
	rls := r.getPreRelease(bundle)

	applyedRelease, err := r.Helm.ApplyChart(ctx, rls.Name, rls.Namespace, into, rls.Config, helm.ApplyOptions{})
	if err != nil {
		return err
	}
	bundle.Status.ManagedResource = parseManagedResource([]byte(applyedRelease.Manifest))
	if applyedRelease.Info.Status != release.StatusDeployed {
		return fmt.Errorf("apply not finished:%s", applyedRelease.Info.Description)
	}
	bundle.Status.Phase = bundlev1.PhaseInstalled
	bundle.Status.Message = applyedRelease.Info.Description
	bundle.Status.CreationTimestamp = convtime(applyedRelease.Info.FirstDeployed.Time)
	bundle.Status.UpgradeTimestamp = convtime(applyedRelease.Info.LastDeployed.Time)
	bundle.Status.Values = MarshalValues(applyedRelease.Config)
	bundle.Status.Version = applyedRelease.Chart.Metadata.Version
	bundle.Status.Notes = applyedRelease.Info.Notes
	return nil
}

func (r *HelmApplier) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	log := logr.FromContextOrDiscard(ctx)

	if bundle.Status.Phase == bundlev1.PhaseNone || bundle.Status.Phase == "" {
		log.Info("already removed or not installed")
		return nil
	}

	rls := r.getPreRelease(bundle)

	// uninstall
	removedRelease, err := r.Helm.RemoveChart(ctx, rls.Name, rls.Namespace, helm.RemoveOptions{})
	if err != nil {
		return err
	}
	log.Info("removed")
	if removedRelease == nil {
		bundle.Status.Phase = bundlev1.PhaseNone
		bundle.Status.Message = "plugin not install"
		return nil
	}
	bundle.Status.Phase = bundlev1.PhaseNone
	bundle.Status.Message = removedRelease.Info.Description
	bundle.Status.Notes = removedRelease.Info.Notes
	bundle.Status.DeletionTimestamp = func() *metav1.Time {
		t := convtime(removedRelease.Info.Deleted.Time)
		return &t
	}()
	return nil
}

func (r HelmApplier) getPreRelease(bundle *bundlev1.Bundle) *release.Release {
	releaseNamespace := bundle.Spec.InstallNamespace
	if releaseNamespace == "" {
		releaseNamespace = bundle.Namespace
	}
	releaseName := bundle.Name
	if helm := bundle.Spec.Helm; helm != nil && helm.ReleaseName != "" {
		releaseName = helm.ReleaseName
	}
	values := UnmarshalValues(bundle.Spec.Values)
	return &release.Release{Name: releaseName, Namespace: releaseNamespace, Config: values}
}

// https://github.com/golang/go/issues/19502
// metav1.Time and time.Time are not comparable directly
func convtime(t time.Time) metav1.Time {
	t, _ = time.Parse(time.RFC3339, t.Format(time.RFC3339))
	return metav1.Time{Time: t}
}

func parseManagedResource(resources []byte) []bundlev1.ManagedResource {
	ress, _ := SplitYAML(resources)
	var managedResources []bundlev1.ManagedResource
	for _, res := range ress {
		managedResources = append(managedResources, bundlev1.ManagedResource{
			APIVersion: res.GetAPIVersion(),
			Kind:       res.GetKind(),
			Name:       res.GetName(),
			Namespace:  res.GetNamespace(),
		})
	}
	return managedResources
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
