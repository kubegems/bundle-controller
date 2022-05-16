package bundle

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/helm"
)

var _ PluginInstaller = &HelmPlugin{}

type HelmPlugin struct {
	Helm *helm.Helm
}

func NewHelmPlugin(config *rest.Config) *HelmPlugin {
	return &HelmPlugin{Helm: &helm.Helm{Config: config}}
}

func (r *HelmPlugin) Apply(ctx context.Context, plugin *Plugin) error {
	if plugin.DownloadTo == "" {
		plugin.DownloadTo = plugin.Name
	}
	applyedRelease, err := r.Helm.ApplyChart(ctx, plugin.Name, plugin.Namespace, plugin.DownloadTo, plugin.Values, helm.ApplyOptions{})
	if err != nil {
		return err
	}
	plugin.Status.Managed = parseManagedResource([]byte(applyedRelease.Manifest))
	if applyedRelease.Info.Status != release.StatusDeployed {
		return fmt.Errorf("apply not finished:%s", applyedRelease.Info.Description)
	}
	plugin.Status.Phase = bundlev1.PhaseInstalled
	plugin.Status.Message = applyedRelease.Info.Description
	plugin.Status.CreationTimestamp = convtime(applyedRelease.Info.FirstDeployed.Time)
	plugin.Status.LastUpdateTimestamp = convtime(applyedRelease.Info.LastDeployed.Time)
	return nil
}

func (r *HelmPlugin) Remove(ctx context.Context, plugin *Plugin) error {
	log := logr.FromContextOrDiscard(ctx)
	if plugin.Status.Phase == bundlev1.PhaseNone {
		log.Info("already removed")
		return nil
	}
	if plugin.Status.Phase == "" {
		plugin.Status.Phase = bundlev1.PhaseNone
		plugin.Status.Message = "plugin not install"
		return nil
	}
	// uninstall
	removedRelease, err := r.Helm.RemoveChart(ctx, plugin.Name, plugin.Namespace, helm.RemoveOptions{})
	if err != nil {
		return err
	}
	if removedRelease == nil {
		plugin.Status.Phase = bundlev1.PhaseNone
		plugin.Status.Message = "plugin not install"
		return nil
	}
	plugin.Status.Managed = parseManagedResource([]byte(removedRelease.Manifest))
	plugin.Status.Phase = bundlev1.PhaseNone
	plugin.Status.Message = removedRelease.Info.Description
	plugin.Status.DeletionTimestamp = func() *metav1.Time {
		t := convtime(removedRelease.Info.Deleted.Time)
		return &t
	}()
	return nil
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

func SplitYAML(data []byte) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(u); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}
