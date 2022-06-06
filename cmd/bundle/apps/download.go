package apps

import (
	"bytes"
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		objs, err := utils.SplitYAMLFilterd[*bundlev1.Bundle](bytes.NewReader(content))
		if err != nil {
			return err
		}
		for _, obj := range objs {
			if err := fun(obj); err != nil {
				return err
			}
		}
	}
	return nil
}
