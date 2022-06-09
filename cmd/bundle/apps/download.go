package apps

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
	"kubegems.io/bundle-controller/pkg/utils"
)

func init() {
	bundlev1.AddToScheme(scheme.Scheme)
}

func NewDownloadCmd(options *bundle.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "download a bundle",
		Example: `
# download a helm bundle into bundles
bundle -c bundles download helm-bundle.yaml
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			zapl, _ := zap.NewDevelopment()
			ctx = logr.NewContext(ctx, zapr.NewLogger(zapl))

			apply := bundle.NewDefaultApply(nil, nil, options)

			return forBundleInPathes(args, func(bundle *bundlev1.Bundle) error {
				_, err := apply.Download(ctx, bundle)
				return err
			})
		},
	}
	return cmd
}

func forBundleInPathes(pathes []string, fun func(*bundlev1.Bundle) error) error {
	if len(pathes) == 1 && pathes[0] == "-" {
		objs, err := utils.SplitYAMLFilterd[*bundlev1.Bundle](os.Stdin)
		if err != nil {
			return err
		}
		for _, obj := range objs {
			if err := fun(obj); err != nil {
				return err
			}
		}
		return nil
	}

	for _, path := range pathes {
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}

		var objs []*bundlev1.Bundle
		if fi.IsDir() {
			objs = []*bundlev1.Bundle{bundleFromDir(path)}
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			objs, err = utils.SplitYAMLFilterd[*bundlev1.Bundle](bytes.NewReader(content))
			if err != nil {
				return err
			}
		}
		for _, obj := range objs {
			if err := fun(obj); err != nil {
				return err
			}
		}
	}
	return nil
}

func bundleFromDir(dir string) *bundlev1.Bundle {
	// detect kind
	kind := bundlev1.BundleKindTemplate
	if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err == nil {
		kind = bundlev1.BundleKindHelm
	} else if _, err := os.Stat(filepath.Join(dir, "kustomization.yaml")); err == nil {
		kind = bundlev1.BundleKindKustomize
	}
	dir, _ = filepath.Abs(dir)
	return &bundlev1.Bundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      filepath.Base(dir),
			Namespace: "default",
		},
		Spec: bundlev1.BundleSpec{
			Kind: kind,
			URL:  "file://" + dir,
		},
	}
}
