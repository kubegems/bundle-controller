package helm

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/utils"
)

type Apply struct {
	Config *rest.Config
}

func New(config *rest.Config) *Apply {
	return &Apply{Config: config}
}

func (r *Apply) Template(ctx context.Context, bundle *bundlev1.Bundle, into string) ([]byte, error) {
	rls := r.getPreRelease(bundle)
	applyedRelease, err := r.ApplyChart(ctx, rls.Name, rls.Namespace, into, rls.Config, ApplyOptions{DryRun: true})
	if err != nil {
		return nil, err
	}
	return []byte(applyedRelease.Manifest), nil
}

func (r *Apply) Apply(ctx context.Context, bundle *bundlev1.Bundle, into string) error {
	rls := r.getPreRelease(bundle)
	applyedRelease, err := r.ApplyChart(ctx, rls.Name, rls.Namespace, into, rls.Config, ApplyOptions{})
	if err != nil {
		return err
	}
	bundle.Status.Resources = parseResource([]byte(applyedRelease.Manifest))
	if applyedRelease.Info.Status != release.StatusDeployed {
		return fmt.Errorf("apply not finished:%s", applyedRelease.Info.Description)
	}
	bundle.Status.Phase = bundlev1.PhaseInstalled
	bundle.Status.Message = applyedRelease.Info.Notes
	bundle.Status.Namespace = applyedRelease.Namespace
	bundle.Status.CreationTimestamp = convtime(applyedRelease.Info.FirstDeployed.Time)
	bundle.Status.UpgradeTimestamp = convtime(applyedRelease.Info.LastDeployed.Time)
	bundle.Status.Values = bundlev1.Values{Object: applyedRelease.Config}
	bundle.Status.Version = applyedRelease.Chart.Metadata.Version
	return nil
}

func (r *Apply) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	log := logr.FromContextOrDiscard(ctx)
	if bundle.Status.Phase == bundlev1.PhaseNone || bundle.Status.Phase == "" {
		log.Info("already removed or not installed")
		return nil
	}
	rls := r.getPreRelease(bundle)
	// uninstall
	removedRelease, err := r.RemoveChart(ctx, rls.Name, rls.Namespace, RemoveOptions{})
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
	return nil
}

func (r Apply) getPreRelease(bundle *bundlev1.Bundle) *release.Release {
	releaseNamespace := bundle.Spec.InstallNamespace
	if releaseNamespace == "" {
		releaseNamespace = bundle.Namespace
	}
	return &release.Release{Name: bundle.Name, Namespace: releaseNamespace, Config: bundle.Spec.Values.Object}
}

// https://github.com/golang/go/issues/19502
// metav1.Time and time.Time are not comparable directly
func convtime(t time.Time) metav1.Time {
	t, _ = time.Parse(time.RFC3339, t.Format(time.RFC3339))
	return metav1.Time{Time: t}
}

func parseResource(resources []byte) []bundlev1.ManagedResource {
	ress, _ := utils.SplitYAML(resources)
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
