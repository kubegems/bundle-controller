package kustomize

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SyncOptions struct {
	ServerSideApply bool
	CreateNamespace bool
}

func (a *Apply) Sync(
	ctx context.Context,
	diff DiffResult,
	options *SyncOptions,
) ([]bundlev1.ManagedResource, error) {
	log := logr.FromContextOrDiscard(ctx)

	errs := []string{}

	managed := []bundlev1.ManagedResource{}
	// create
	for _, item := range diff.Creats {
		log.Info("creating resource", "resource", item.GetObjectKind().GroupVersionKind().String(), "name", item.GetName(), "namespace", item.GetNamespace())
		if options.CreateNamespace {
			a.createNsIfNotExists(ctx, item.GetNamespace())
		}
		if err := a.apply(ctx, item, options.ServerSideApply); err != nil {
			err = fmt.Errorf("%s %s/%s: %v", item.GetObjectKind().GroupVersionKind().String(), item.GetNamespace(), item.GetName(), err)
			log.Error(err, "creating resource")
			errs = append(errs, err.Error())
			continue
		}
		managed = append(managed, manFromResource(item)) // set managed
	}

	// apply
	for _, item := range diff.Applys {
		managed = append(managed, manFromResource(item)) // set managed

		log.Info("applying resource", "resource", item.GetObjectKind().GroupVersionKind().String(), "name", item.GetName(), "namespace", item.GetNamespace())
		if options.CreateNamespace {
			a.createNsIfNotExists(ctx, item.GetNamespace())
		}
		if err := a.apply(ctx, item, options.ServerSideApply); err != nil {
			err = fmt.Errorf("%s %s/%s: %v", item.GetObjectKind().GroupVersionKind().String(), item.GetNamespace(), item.GetName(), err)
			log.Error(err, "applying resource")
			errs = append(errs, err.Error())
			continue
		}
	}
	// remove
	for _, item := range diff.Removes {
		partial := item
		log.Info("deleting resource", "gvk", partial.GetObjectKind().GroupVersionKind().String(), "name", partial.GetName(), "namespace", partial.GetNamespace())
		if err := a.Client.Delete(ctx, partial, &client.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				err = fmt.Errorf("%s %s/%s: %v", partial.GetObjectKind().GroupVersionKind().String(), partial.GetNamespace(), partial.GetName(), err)
				log.Error(err, "deleting resource")
				errs = append(errs, err.Error())
				// if not removed, keep in managed
				managed = append(managed, manFromResource(item)) // set managed
				continue
			}
		}
	}

	// sort manged
	sort.Slice(managed, func(i, j int) bool {
		return strings.Compare(managed[i].APIVersion, managed[j].APIVersion) == 1
	})
	if len(errs) > 0 {
		return managed, errors.New(strings.Join(errs, "\n"))
	} else {
		return managed, nil
	}
}

func manFromResource(obj client.Object) bundlev1.ManagedResource {
	return bundlev1.ManagedResource{
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
	}
}

func partialFromMan(man bundlev1.ManagedResource) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: man.APIVersion,
			Kind:       man.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      man.Name,
			Namespace: man.Namespace,
		},
	}
}

func (a *Apply) createNsIfNotExists(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, a.Client, ns, func() error { return nil })
	return err
}

func (a *Apply) apply(ctx context.Context, obj client.Object, serversideapply bool) error {
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
