package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	FinalizerName = "bundle.kubegems.io/finalizer"
)

//+kubebuilder:rbac:groups=bundle.kubegems.io,resources=bundles,verbs=*
type BundleReconciler struct {
	client.Client
	BundleManager bundle.PluginInstaller
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

func Setup(mgr ctrl.Manager) error {
	bc := &BundleReconciler{
		Client:        mgr.GetClient(),
		BundleManager: bundle.NewDelegateManager(mgr),
	}
	return bc.SetupWithManager(mgr)
}

func (r *BundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bundlev1.Bundle{}).
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
func (r *BundleReconciler) sync(ctx context.Context, plugin *bundlev1.Bundle) error {
	shouldRemove := plugin.DeletionTimestamp != nil

	// nolint: nestif
	if !shouldRemove && len(plugin.Spec.Dependencies) > 0 {
		// check all dependencies are installed
		for _, dep := range plugin.Spec.Dependencies {
			name, namespace, version := dep.Name, dep.Namespace, dep.Version
			if namespace == "" {
				namespace = plugin.Namespace
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
		return r.remove(ctx, plugin)
	} else {
		return r.apply(ctx, plugin)
	}
}

func (r *BundleReconciler) apply(ctx context.Context, app *bundlev1.Bundle) error {
	spec := bundle.PluginFromPlugin(app)
	if err := r.BundleManager.Apply(ctx, spec); err != nil {
		return err
	}
	app.Status = spec.ToPluginStatus()
	return nil
}

func (r *BundleReconciler) remove(ctx context.Context, app *bundlev1.Bundle) error {
	spec := bundle.PluginFromPlugin(app)
	if err := r.BundleManager.Remove(ctx, spec); err != nil {
		return err
	}
	app.Status = spec.ToPluginStatus()
	return nil
}
