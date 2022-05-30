package kustomize

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

type Apply struct {
	Client client.Client
}

func New(cli client.Client) *Apply {
	return &Apply{Client: cli}
}

func KustomizeBuild(ctx context.Context, dir string) ([]byte, error) {
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	m, err := k.Run(filesys.MakeFsOnDisk(), dir)
	if err != nil {
		return nil, err
	}
	yml, err := m.AsYaml()
	if err != nil {
		return nil, err
	}
	return []byte(yml), nil
}

func (p *Apply) Template(ctx context.Context, bundle *bundlev1.Bundle, into string) ([]byte, error) {
	return KustomizeBuild(ctx, into)
}

func (p *Apply) Apply(ctx context.Context, bundle *bundlev1.Bundle, into string) error {
	log := logr.FromContextOrDiscard(ctx)

	renderd, err := p.Template(ctx, bundle, into)
	if err != nil {
		return err
	}
	resources, err := utils.SplitYAML(renderd)
	if err != nil {
		return err
	}

	ns := bundle.Spec.InstallNamespace
	if ns == "" {
		ns = bundle.Namespace
	}
	// override namespace
	setNamespaceIfNotSet(ns, resources)

	diffresult := Diff(bundle.Status.Resources, resources)
	if len(diffresult.Creats) == 0 && len(diffresult.Removes) == 0 && bundle.Status.Phase == bundlev1.PhaseInstalled {
		log.Info("all resources are already applied")
		return nil
	}

	managedResources, err := p.Sync(ctx, diffresult, &SyncOptions{ServerSideApply: true})
	if err != nil {
		return err
	}
	bundle.Status.Resources = managedResources
	bundle.Status.Phase = bundlev1.PhaseInstalled
	bundle.Status.Version = bundle.Spec.Version
	bundle.Status.Namespace = ns
	now := metav1.Now()
	bundle.Status.UpgradeTimestamp = now
	if bundle.Status.CreationTimestamp.IsZero() {
		bundle.Status.CreationTimestamp = now
	}
	bundle.Status.Message = ""
	return nil
}

type DiffResult struct {
	Creats  []*unstructured.Unstructured
	Applys  []*unstructured.Unstructured
	Removes []*unstructured.Unstructured
}

func Diff(managed []bundlev1.ManagedResource, resources []*unstructured.Unstructured) DiffResult {
	result := DiffResult{}

	managedmap := map[bundlev1.ManagedResource]bool{}
	for _, item := range managed {
		managedmap[item] = false
	}
	for _, item := range resources {
		man := manFromResource(item)
		if _, ok := managedmap[man]; !ok {
			result.Creats = append(result.Creats, item)
		} else {
			result.Applys = append(result.Applys, item)
		}
		managedmap[man] = true
	}
	for k, v := range managedmap {
		if !v {
			uns := &unstructured.Unstructured{}
			uns.SetAPIVersion(k.APIVersion)
			uns.SetKind(k.Kind)
			uns.SetName(k.Name)
			uns.SetNamespace(k.Namespace)
			result.Removes = append(result.Removes, uns)
		}
	}
	return result
}

func (p *Apply) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	diff := Diff(bundle.Status.Resources, nil)
	managedResources, err := p.Sync(ctx, diff, &SyncOptions{ServerSideApply: true})
	if err != nil {
		return err
	}
	bundle.Status.Resources = managedResources
	bundle.Status.Phase = bundlev1.PhaseNone
	bundle.Status.Message = ""
	return nil
}

func setNamespaceIfNotSet(ns string, list []*unstructured.Unstructured) {
	for _, item := range list {
		if item.GetNamespace() == "" {
			item.SetNamespace(ns)
		}
	}
}
