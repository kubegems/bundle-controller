package controllers

import (
	"context"
	"errors"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

type KustomizeApplier struct {
	Client client.Client
}

func NewKustomize(cli client.Client) *KustomizeApplier {
	return &KustomizeApplier{Client: cli}
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

func (p *KustomizeApplier) Apply(ctx context.Context, bundle *bundlev1.Bundle, into string) error {
	if bundle.Status.Phase == bundlev1.PhaseInstalled {
		return nil
	}

	renderd, err := KustomizeBuild(ctx, into)
	if err != nil {
		return err
	}
	resources, err := SplitYAML(renderd)
	if err != nil {
		return err
	}
	// override namespace
	setNamespaceIfNotSet(bundle, resources)

	managedResources, err := p.Sync(ctx, resources, bundle.Status.ManagedResource, true)
	if err != nil {
		return err
	}
	bundle.Status.ManagedResource = managedResources
	bundle.Status.Phase = bundlev1.PhaseInstalled
	now := metav1.Now()
	bundle.Status.UpgradeTimestamp = now
	if bundle.Status.CreationTimestamp.IsZero() {
		bundle.Status.CreationTimestamp = now
	}
	bundle.Status.Message = ""
	return nil
}

func (p *KustomizeApplier) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	managedResources, err := p.Sync(ctx, nil, bundle.Status.ManagedResource, true)
	if err != nil {
		return err
	}
	bundle.Status.ManagedResource = managedResources
	bundle.Status.Phase = bundlev1.PhaseNone
	bundle.Status.Message = ""
	now := metav1.Now()
	bundle.Status.DeletionTimestamp = &now
	return nil
}

func setNamespaceIfNotSet(bundle *bundlev1.Bundle, list []*unstructured.Unstructured) {
	ns := bundle.Spec.InstallNamespace
	if ns != "" {
		ns = bundle.Namespace
	}
	for _, item := range list {
		if item.GetNamespace() == "" {
			item.SetNamespace(ns)
		}
	}
}

func (a *KustomizeApplier) Sync(ctx context.Context, resources []*unstructured.Unstructured, managed []bundlev1.ManagedResource, serverSideApply bool) ([]bundlev1.ManagedResource, error) {
	pruned := diff(resources, managed)

	errs := []string{}
	// apply
	managedResources := make([]bundlev1.ManagedResource, 0, len(resources))
	for _, item := range resources {
		managedResources = append(managedResources, bundlev1.ManagedResource{
			APIVersion: item.GetAPIVersion(),
			Kind:       item.GetKind(),
			Namespace:  item.GetNamespace(),
			Name:       item.GetName(),
		})
		if err := a.apply(ctx, item, serverSideApply); err != nil {
			errs = append(errs, err.Error())
		}
	}
	// remove
	for _, item := range pruned {
		partial := &metav1.PartialObjectMetadata{
			TypeMeta: metav1.TypeMeta{
				APIVersion: item.APIVersion,
				Kind:       item.Kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.Name,
				Namespace: item.Namespace,
			},
		}

		if err := a.Client.Delete(ctx, partial, &client.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	} else {
		return managedResources, nil
	}
}

func (a *KustomizeApplier) apply(ctx context.Context, obj client.Object, serversideapply bool) error {
	key := client.ObjectKeyFromObject(obj)
	if err := a.Client.Get(ctx, key, obj); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// create
		if err := a.Client.Create(ctx, obj); err != nil {
			return err
		}
		return nil
	}

	var patch client.Patch
	var patchoptions []client.PatchOption
	if serversideapply {
		obj.SetManagedFields(nil)
		patch = client.Apply
		patchoptions = append(patchoptions,
			client.FieldOwner("bundler"),
			client.ForceOwnership,
		)
	} else {
		patch = client.MergeFrom(obj)
	}

	// patch
	if err := a.Client.Patch(ctx, obj, patch, patchoptions...); err != nil {
		return err
	}
	return nil
}

func diff[T client.Object](a []T, b []bundlev1.ManagedResource) []bundlev1.ManagedResource {
	for _, item := range a {
		if i := indexOf(item, b); i >= 0 {
			b = append(b[:i], b[i+1:]...)
		}
	}
	return b
}

func indexOf(item client.Object, list []bundlev1.ManagedResource) int {
	for i, l := range list {
		if l.APIVersion == item.GetObjectKind().GroupVersionKind().GroupVersion().Identifier() &&
			l.Kind == item.GetObjectKind().GroupVersionKind().Kind &&
			l.Namespace == item.GetNamespace() &&
			l.Name == item.GetName() {
			return i
		}
	}
	return -1
}
