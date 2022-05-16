package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type BundleApplier struct {
	helm      *HelmApplier
	kustomize *KustomizeApplier
}

func NewApplier(mgr ctrl.Manager) *BundleApplier {
	return &BundleApplier{
		helm:      NewHelm(mgr.GetConfig()),
		kustomize: NewKustomize(mgr.GetClient()),
	}
}

func (b *BundleApplier) Apply(ctx context.Context, bundle *bundlev1.Bundle) error {
	into, err := b.download(ctx, bundle, "")
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	detectKind(into, bundle)
	bundle.Status.Kind = bundle.Spec.Kind

	switch bundle.Spec.Kind {
	case bundlev1.BundleKindHelm:
		return b.helm.Apply(ctx, bundle, into)
	case bundlev1.BundleKindKustomize:
		return b.kustomize.Apply(ctx, bundle, into)
	default:
		return fmt.Errorf("unknown bundle kind: %s", bundle.Spec.Kind)
	}
}

func (b *BundleApplier) Remove(ctx context.Context, bundle *bundlev1.Bundle) error {
	kind := bundle.Spec.Kind
	if kind == "" {
		kind = bundle.Status.Kind
	}
	switch kind {
	case bundlev1.BundleKindHelm:
		return b.helm.Remove(ctx, bundle)
	case bundlev1.BundleKindKustomize:
		return b.kustomize.Remove(ctx, bundle)
	default:
		return fmt.Errorf("unknown bundle kind: %s", kind)
	}
}

func detectKind(path string, bundle *bundlev1.Bundle) {
	if bundle.Spec.Kind != "" {
		return
	}
	if exists(path, "Chart.yaml") {
		bundle.Spec.Kind = bundlev1.BundleKindHelm
		return
	}
	if exists(path, "kustomization.yaml") {
		bundle.Spec.Kind = bundlev1.BundleKindKustomize
		return
	}
}

func exists(path, filename string) bool {
	_, err := os.Stat(filepath.Join(path, filename))
	return !os.IsNotExist(err)
}

func SplitYAML(data []byte) ([]*unstructured.Unstructured, error) {
	d := kubeyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := d.Decode(u); err != nil {
			if err == io.EOF {
				break
			}
			return objs, fmt.Errorf("failed to unmarshal manifest: %v", err)
		}
		objs = append(objs, u)
	}
	return objs, nil
}
