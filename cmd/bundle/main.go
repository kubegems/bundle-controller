package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/scheme"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle"
	"kubegems.io/bundle-controller/pkg/controllers"
	"kubegems.io/bundle-controller/pkg/utils"
)

const ErrExitCode = 1

func init() {
	bundlev1.AddToScheme(scheme.Scheme)
}

func main() {
	if err := NewBundleControllerCmd().Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(ErrExitCode)
	}
}

func NewBundleControllerCmd() *cobra.Command {
	globalOptions := bundle.NewDefaultOptions()
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "commands of bundle",
	}
	cmd.AddCommand(
		NewRunCmd(globalOptions),
		NewDownloadCmd(globalOptions),
		NewTemplateCmd(globalOptions),
	)
	cmd.PersistentFlags().StringVarP(&globalOptions.CacheDir, "cache-dir", "c", globalOptions.CacheDir, "cache directory")
	cmd.PersistentFlags().StringSliceVarP(&globalOptions.SearchDirs, "search-dir", "s", globalOptions.SearchDirs, "search bundles in directory")
	return cmd
}

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
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true}, // ignore unknown flags errors
	}
	cmd.Flags().StringVarP(&options.MetricsAddr, "metrics-addr", "", options.MetricsAddr, "metrics address")
	cmd.Flags().StringVarP(&options.ProbeAddr, "probe-addr", "", options.ProbeAddr, "probe address")
	cmd.Flags().BoolVarP(&options.EnableLeaderElection, "enable-leader-election", "", options.EnableLeaderElection, "enable leader election")
	return cmd
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

func DetectPluginType(path string) bundlev1.BundleKind {
	// helm ?
	if _, err := os.Stat(filepath.Join(path, "Chart.yaml")); err == nil {
		return bundlev1.BundleKindHelm
	}
	// kustomize ?
	if _, err := os.Stat(filepath.Join(path, "kustomization.yaml")); err == nil {
		return bundlev1.BundleKindKustomize
	}
	return bundlev1.BundleKindUnknown
}
