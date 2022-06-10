package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/strvals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

const MaxConcurrentReconciles = 5

const (
	FinalizerName = "bundle.kubegems.io/finalizer"
)

//+kubebuilder:rbac:groups=bundle.kubegems.io,resources=bundles,verbs=*
type BundleReconciler struct {
	client.Client
	Applier *bundle.BundleApplier
}

func (r *BundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logr.FromContextOrDiscard(ctx)
	app := &bundlev1.Bundle{}
	if err := r.Client.Get(ctx, req.NamespacedName, app); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// The object is not being deleted, so if it does not have our finalizer,
	// then lets add the finalizer and update the object. This is equivalent
	// registering our finalizer.
	if app.DeletionTimestamp == nil && !controllerutil.ContainsFinalizer(app, FinalizerName) {
		log.Info("add finalizer")
		controllerutil.AddFinalizer(app, FinalizerName)
		if err := r.Update(ctx, app); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// check the object is being deleted then remove the finalizer
	if app.DeletionTimestamp != nil && controllerutil.ContainsFinalizer(app, FinalizerName) {
		if app.Status.Phase == bundlev1.PhaseFailed || app.Status.Phase == bundlev1.PhaseDisabled {
			controllerutil.RemoveFinalizer(app, FinalizerName)
			if err := r.Update(ctx, app); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Info("waiting for app to be removed, then remove finalizer")
	}

	// sync
	err := r.Sync(ctx, app)
	if err != nil {
		app.Status.Phase = bundlev1.PhaseFailed
		app.Status.Message = err.Error()
	}
	// update status if updated whenever the sync has error or no
	if err := r.Status().Update(ctx, app); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, err
}

func Setup(ctx context.Context, mgr ctrl.Manager, options *bundle.Options) error {
	cfg, cli := mgr.GetConfig(), mgr.GetClient()
	r := &BundleReconciler{
		Client:  cli,
		Applier: bundle.NewDefaultApply(cfg, cli, options),
	}
	handler := ConfigMapOrSecretTrigger(ctx, cli)
	return ctrl.NewControllerManagedBy(mgr).
		For(&bundlev1.Bundle{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: MaxConcurrentReconciles}).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler).
		Complete(r)
}

// Sync
func (r *BundleReconciler) Sync(ctx context.Context, bundle *bundlev1.Bundle) error {
	if bundle.Spec.Disabled || bundle.DeletionTimestamp != nil {
		// just remove
		return r.Applier.Remove(ctx, bundle)
	} else {
		// check all dependencies are installed
		if err := r.checkDepenency(ctx, bundle); err != nil {
			return err
		}
		// resolve valuesRef
		if err := r.resolveValuesRef(ctx, bundle); err != nil {
			return err
		}
		return r.Applier.Apply(ctx, bundle)
	}
}

type DependencyError struct {
	Reason string
	Object corev1.ObjectReference
}

func (e DependencyError) Error() string {
	return fmt.Sprintf("dependency %s/%s :%s", e.Object.Namespace, e.Object.Name, e.Reason)
}

func (r *BundleReconciler) checkDepenency(ctx context.Context, bundle *bundlev1.Bundle) error {
	for _, dep := range bundle.Spec.Dependencies {
		if dep.Name == "" {
			continue
		}
		if dep.Namespace == "" {
			dep.Namespace = bundle.Namespace
		}
		if dep.Kind == "" {
			dep.APIVersion = bundle.APIVersion
			dep.Kind = bundle.Kind
		}
		newobj, _ := r.Scheme().New(dep.GroupVersionKind())
		depobj, ok := newobj.(client.Object)
		if !ok {
			depobj = &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					APIVersion: dep.GroupVersionKind().GroupVersion().String(),
					Kind:       dep.Kind,
				},
			}
		}

		// exists check
		if err := r.Client.Get(ctx, client.ObjectKey{Namespace: dep.Namespace, Name: dep.Name}, depobj); err != nil {
			if apierrors.IsNotFound(err) {
				return DependencyError{Reason: err.Error(), Object: dep}
			}
			return err
		}

		// status check
		switch obj := depobj.(type) {
		case *bundlev1.Bundle:
			if obj.Status.Phase != bundlev1.PhaseInstalled {
				return DependencyError{Reason: "not installed", Object: dep}
			}
		}
	}
	return nil
}

func (r *BundleReconciler) resolveValuesRef(ctx context.Context, bundle *bundlev1.Bundle) error {
	base := map[string]interface{}{}

	for _, ref := range bundle.Spec.ValuesFrom {
		switch strings.ToLower(ref.Kind) {
		case "secret", "secrets":
			secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: bundle.Namespace}}
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
				if ref.Optional && apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			// --set
			for k, v := range secret.Data {
				if err := mergeInto(ref.Prefix+k, string(v), base); err != nil {
					return fmt.Errorf("parse %#v key[%s]: %w", ref, k, err)
				}
			}
		case "configmap", "configmaps":
			configmap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: bundle.Namespace}}
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(configmap), configmap); err != nil {
				if ref.Optional && apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
			// -f/--values
			for k, v := range configmap.BinaryData {
				currentMap := map[string]interface{}{}
				if err := yaml.Unmarshal(v, &currentMap); err != nil {
					return fmt.Errorf("parse %#v key[%s]: %w", ref, k, err)
				}
				base = mergeMaps(base, currentMap)
			}
			// --set
			for k, v := range configmap.Data {
				if err := mergeInto(ref.Prefix+k, string(v), base); err != nil {
					return fmt.Errorf("parse %#v key[%s]: %w", ref, k, err)
				}
			}
		default:
			return fmt.Errorf("valuesRef kind [%s] is not supported", ref.Kind)
		}
	}

	// inlined values
	base = mergeMaps(base, bundle.Spec.Values.Object)

	bundle.Spec.Values = bundlev1.Values{Object: base}.FullFill()
	return nil
}

func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

func mergeInto(k, v string, base map[string]interface{}) error {
	if err := strvals.ParseInto(fmt.Sprintf("%s=%s", k, v), base); err != nil {
		return fmt.Errorf("parse %#v key[%s]: %w", k, v, err)
	}
	return nil
}
