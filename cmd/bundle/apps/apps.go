package apps

import (
	"github.com/spf13/cobra"
	"kubegems.io/bundle-controller/pkg/bundle"
)

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
