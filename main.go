package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"kubegems.io/bundle-controller/pkg/controllers"
)

const ErrExitCode = 1

func main() {
	if err := NewBundleControllerCmd().Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(ErrExitCode)
	}
}

func NewBundleControllerCmd() *cobra.Command {
	options := controllers.NewDefaultOptions()
	cmd := &cobra.Command{
		Use:   "bundle-controller",
		Short: "controller for bundle resources like helm and kustomize",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer cancel()
			return controllers.Run(ctx, options)
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true}, // ignore unknown flags errors
	}
	return cmd
}
