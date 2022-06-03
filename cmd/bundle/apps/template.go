package apps

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
)

func NewTemplateCmd(options *bundle.Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "template a bundle",
		Example: `
# template a helm bundle into stdout
bundle -c bundles template helm-bundle.yaml
		`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			apply := bundle.NewDefaultApply(nil, nil, options)
			return forBundleInPathes(args, func(bundle *bundlev1.Bundle) error {
				content, err := apply.Template(ctx, bundle)
				if err != nil {
					return err
				}
				fmt.Print(string(content))
				return nil
			})
		},
	}
	return cmd
}