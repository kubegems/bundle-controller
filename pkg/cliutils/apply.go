package cliutils

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Applier struct {
	Client client.Client
}

type ObjectMeta struct {
	schema.GroupVersionKind
	Namespace string
	Name      string
}

type SyncResult struct {
	Object ObjectMeta
	Error  error
}

type Options struct {
	Dryrun          string
	ServerSideApply bool
}

func (a *Applier) Sync(ctx context.Context, resources []*unstructured.Unstructured, managed []ObjectMeta, options Options) ([]SyncResult, error) {
	results := []SyncResult{}
	pruned := diff(resources, managed)

	// apply
	for _, item := range resources {
		err := a.apply(ctx, item, options.ServerSideApply)
		syncresult := SyncResult{
			Object: ObjectMeta{
				GroupVersionKind: item.GroupVersionKind(), Namespace: item.GetNamespace(), Name: item.GetName(),
			},
		}
		if err != nil {
			syncresult.Error = err
		}
		results = append(results, syncresult)

	}

	// remove
	for _, item := range pruned {
		partial := &metav1.PartialObjectMetadata{}
		partial.GetObjectKind().SetGroupVersionKind(item.GroupVersionKind)
		partial.SetNamespace(item.Namespace)
		partial.SetName(item.Name)

		syncresult := SyncResult{Object: item}
		if err := a.Client.Delete(ctx, partial, &client.DeleteOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				syncresult.Error = err
			}
		}
		results = append(results, syncresult)
	}

	return results, nil
}

func (a *Applier) apply(ctx context.Context, obj client.Object, serversideapply bool) error {
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

func diff[T client.Object](a []T, b []ObjectMeta) []ObjectMeta {
	for _, item := range a {
		if i := indexOf(item, b); i >= 0 {
			b = append(b[:i], b[i+1:]...)
		}
	}
	return b
}

func indexOf(item client.Object, list []ObjectMeta) int {
	for i, l := range list {
		if l.GroupVersionKind == item.GetObjectKind().GroupVersionKind() &&
			l.Namespace == item.GetNamespace() &&
			l.Name == item.GetName() {
			return i
		}
	}
	return -1
}
