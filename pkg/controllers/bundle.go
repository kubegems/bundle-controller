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
	"k8s.io/apimachinery/pkg/types"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const MaxConcurrentReconciles = 5

const (
	FinalizerName = "bundle.kubegems.io/finalizer"
)

//+kubebuilder:rbac:groups=bundle.kubegems.io,resources=bundles,verbs=*
type BundleReconciler struct {
	client.Client
	applier *bundle.BundleApplier
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
		if app.Status.Phase == bundlev1.PhaseFailed || app.Status.Phase == bundlev1.PhaseNone {
			controllerutil.RemoveFinalizer(app, FinalizerName)
			if err := r.Update(ctx, app); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		log.Info("waiting for app to be removed, then remove finalizer")
	}

	// sync
	err := r.sync(ctx, app)
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

func Setup(mgr ctrl.Manager, options *bundle.Options) error {
	r := &BundleReconciler{
		Client:  mgr.GetClient(),
		applier: bundle.NewDefaultApply(mgr.GetConfig(), mgr.GetClient(), options),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&bundlev1.Bundle{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: MaxConcurrentReconciles}).
		Complete(r)
}

type DependencyError struct {
	Reason     string
	Dependency bundlev1.Dependency
}

func (e DependencyError) Error() string {
	return fmt.Sprintf("dependency %s/%s :%s", e.Dependency.Namespace, e.Dependency.Name, e.Reason)
}

// sync
func (r *BundleReconciler) sync(ctx context.Context, bundle *bundlev1.Bundle) error {
	shouldRemove := bundle.DeletionTimestamp != nil
	// nolint: nestif
	if !shouldRemove && len(bundle.Spec.Dependencies) > 0 {
		// check all dependencies are installed
		for _, dep := range bundle.Spec.Dependencies {
			name, namespace, version := dep.Name, dep.Namespace, dep.Version
			if namespace == "" {
				namespace = bundle.Namespace
			}
			if name == "" {
				continue
			}
			depbundle := &bundlev1.Bundle{}
			if err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, depbundle); err != nil {
				if apierrors.IsNotFound(err) {
					return DependencyError{Reason: "not found", Dependency: dep}
				}
				return err
			}
			if depbundle.Status.Phase != bundlev1.PhaseInstalled {
				return DependencyError{Reason: "not installed", Dependency: dep}
			}
			if version != "" {
				// TODO: check version
			}
		}
	}

	if shouldRemove {
		return r.applier.Remove(ctx, bundle)
	} else {
		// resolve valuesRef
		if err := r.resolveValuesRef(ctx, bundle); err != nil {
			return err
		}
		return r.applier.Apply(ctx, bundle)
	}
}

func (r *BundleReconciler) resolveValuesRef(ctx context.Context, bundle *bundlev1.Bundle) error {
	base := map[string]interface{}{}

	for _, ref := range bundle.Spec.ValuesRef {
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
				if err := strvals.ParseInto(fmt.Sprintf("%s=%s", k, string(v)), base); err != nil {
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
				if err := strvals.ParseInto(fmt.Sprintf("%s=%s", k, string(v)), base); err != nil {
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
