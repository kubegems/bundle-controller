package apps

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"kubegems.io/bundle-controller/pkg/bundle"
	"kubegems.io/bundle-controller/pkg/controllers"
)

func NewRunCmd(bundleoptions *bundle.Options) *cobra.Command {
	options := controllers.NewDefaultOptions()
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run  bundle controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return controllers.Run(ctx, options, bundleoptions)
		},
	}
	cmd.Flags().StringVarP(&options.MetricsAddr, "metrics-addr", "", options.MetricsAddr, "metrics address")
	cmd.Flags().StringVarP(&options.ProbeAddr, "probe-addr", "", options.ProbeAddr, "probe address")
	cmd.Flags().BoolVarP(&options.EnableLeaderElection, "enable-leader-election", "", options.EnableLeaderElection, "enable leader election")
	return cmd
}