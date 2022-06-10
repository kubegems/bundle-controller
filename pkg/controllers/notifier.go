package controllers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func ConfigMapOrSecretTrigger(ctx context.Context, cli client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		kind := ""
		switch obj.(type) {
		case *corev1.ConfigMap:
			kind = "ConfigMap"
		case *corev1.Secret:
			kind = "Secret"
		default:
			return nil
		}

		bundles := bundlev1.BundleList{}
		_ = cli.List(ctx, &bundles, client.InNamespace(obj.GetNamespace()))

		var requests []reconcile.Request
		for _, bundle := range bundles.Items {
			for _, ref := range bundle.Spec.ValuesFrom {
				if ref.Kind == kind && ref.Name == obj.GetName() {
					requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&bundle)})
				}
			}
		}
		return requests
	})
}
