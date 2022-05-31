package kustomize

import (
	"context"

	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

func KustomizeBuildFunc(ctx context.Context, bundle *bundlev1.Bundle, dir string) ([]byte, error) {
	return KustomizeBuild(ctx, dir)
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
