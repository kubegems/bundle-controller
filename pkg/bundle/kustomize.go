package bundle

import (
	"context"
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/cliutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

var _ PluginInstaller = &KustomizePlugin{}

type KustomizePlugin struct {
	Applier cliutils.Applier
}

func NewKustomizePlugin(cli client.Client) *KustomizePlugin {
	return &KustomizePlugin{Applier: cliutils.Applier{Client: cli}}
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

func (p *KustomizePlugin) Apply(ctx context.Context, plugin *Plugin) error {
	renderd, err := KustomizeBuild(ctx, plugin.DownloadTo)
	if err != nil {
		return err
	}
	resources, err := SplitYAML(renderd)
	if err != nil {
		return err
	}
	// override namespace
	setNamespaceIfNotSet(plugin.Namespace, resources)

	if err := p.sync(ctx, resources, &plugin.Status); err != nil {
		return err
	}
	plugin.Status.Phase = bundlev1.PhaseInstalled
	now := metav1.Now()
	plugin.Status.LastUpdateTimestamp = now
	if plugin.Status.CreationTimestamp.IsZero() {
		plugin.Status.CreationTimestamp = now
	}
	plugin.Status.Message = ""
	return nil
}

func (p *KustomizePlugin) Remove(ctx context.Context, plugin *Plugin) error {
	if err := p.sync(ctx, nil, &plugin.Status); err != nil {
		return err
	}
	plugin.Status.Phase = bundlev1.PhaseNone
	plugin.Status.Message = ""

	now := metav1.Now()
	plugin.Status.DeletionTimestamp = &now
	return nil
}

func setNamespaceIfNotSet(ns string, list []*unstructured.Unstructured) {
	for _, item := range list {
		if item.GetNamespace() == "" {
			item.SetNamespace(ns)
		}
	}
}

func (p *KustomizePlugin) sync(ctx context.Context, resources []*unstructured.Unstructured, status *PluginStatus) error {
	managed := make([]cliutils.ObjectMeta, 0, len(resources))
	for _, item := range resources {
		managed = append(managed, cliutils.ObjectMeta{
			GroupVersionKind: item.GroupVersionKind(),
			Namespace:        item.GetNamespace(),
			Name:             item.GetName(),
		})
	}
	results, err := p.Applier.Sync(ctx, resources, managed, cliutils.Options{})
	if err != nil {
		return err
	}
	managedResources := make([]bundlev1.ManagedResource, 0, len(results))
	errormsgs := []string{}
	for _, result := range results {
		managedResource := bundlev1.ManagedResource{
			APIVersion: result.Object.GroupVersionKind.GroupVersion().String(),
			Kind:       result.Object.GroupVersionKind.Kind,
			Namespace:  result.Object.Namespace,
			Name:       result.Object.Name,
		}
		if result.Error != nil {
			errmsg := result.Error.Error()
			managedResource.Error = errmsg
			errormsgs = append(errormsgs, errmsg)
		}
	}
	status.Managed = managedResources
	if len(errormsgs) > 0 {
		return errors.New(strings.Join(errormsgs, "\n"))
	}
	return nil
}
