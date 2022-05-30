package bundle

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle/helm"
	"kubegems.io/bundle-controller/pkg/bundle/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Apply interface {
	Apply(ctx context.Context, bundle *bundlev1.Bundle, into string) error
	Remove(ctx context.Context, bundle *bundlev1.Bundle) error
	Template(ctx context.Context, bundle *bundlev1.Bundle, into string) ([]byte, error)
}

type BundleApplier struct {
	Options  *Options
	appliers map[bundlev1.BundleKind]Apply
}

type Options struct {
	CacheDir   string
	SearchDirs []string
}

func NewDefaultOptions() *Options {
	return &Options{}
}

func NewDefaultApply(cfg *rest.Config, cli client.Client, options *Options) *BundleApplier {
	return &BundleApplier{
		Options: options,
		appliers: map[bundlev1.BundleKind]Apply{
			bundlev1.BundleKindHelm:      helm.New(cfg),
			bundlev1.BundleKindKustomize: kustomize.New(cli),
		},
	}
}

func (b *BundleApplier) Template(ctx context.Context, bundle *bundlev1.Bundle) ([]byte, error) {
	into, err := b.Download(ctx, bundle)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	if apply, ok := b.appliers[bundle.Spec.Kind]; ok {
		return apply.Template(ctx, bundle, into)
	}
	return nil, fmt.Errorf("unknown bundle kind: %s", bundle.Spec.Kind)
}

func (b *BundleApplier) Download(ctx context.Context, bundle *bundlev1.Bundle) (string, error) {
	return Download(ctx, bundle, b.Options.CacheDir, b.Options.SearchDirs...)
}

func (b *BundleApplier) Apply(ctx context.Context, bundle *bundlev1.Bundle) error {
	into, err := b.Download(ctx, bundle)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	if apply, ok := b.appliers[bundle.Spec.Kind]; ok {
		return apply.Apply(ctx, bundle, into)
	}
	return fmt.Errorf("unknown bundle kind: %s", bundle.Spec.Kind)
}

func (b *BundleApplier) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	if apply, ok := b.appliers[bundle.Spec.Kind]; ok {
		return apply.Remove(ctx, bundle)
	}
	return fmt.Errorf("unknown bundle kind: %s", bundle.Spec.Kind)
}
