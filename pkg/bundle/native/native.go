package native

import (
	"context"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TemplateFun func(ctx context.Context, bundle *bundlev1.Bundle, into string) ([]byte, error)

type Apply struct {
	TemplateFun TemplateFun
	Cli         *utils.Apply
}

func New(cli client.Client, fun TemplateFun) *Apply {
	return &Apply{
		TemplateFun: fun,
		Cli:         &utils.Apply{Client: cli},
	}
}

func (p *Apply) Template(ctx context.Context, bundle *bundlev1.Bundle, into string) ([]byte, error) {
	return p.TemplateFun(ctx, bundle, into)
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

	diffresult := utils.Diff(bundle.Status.Resources, resources)
	if len(diffresult.Creats) == 0 && len(diffresult.Removes) == 0 && bundle.Status.Phase == bundlev1.PhaseInstalled {
		log.Info("all resources are already applied")
		return nil
	}
	managedResources, err := p.Cli.SyncDiff(ctx, diffresult, utils.NewDefaultSyncOptions())
	if err != nil {
		return err
	}
	bundle.Status.Resources = managedResources
	bundle.Status.Values = bundlev1.Values{Object: bundle.Spec.Values.Object}.FullFill()
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

func (p *Apply) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	managedResources, err := p.Cli.Sync(ctx, bundle.Status.Resources, nil, utils.NewDefaultSyncOptions())
	if err != nil {
		return err
	}
	bundle.Status.Resources = managedResources
	bundle.Status.Phase = bundlev1.PhaseDisabled
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
