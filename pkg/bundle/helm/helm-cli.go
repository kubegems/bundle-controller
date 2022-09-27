package helm

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"kubegems.io/bundle-controller/pkg/utils"
)

type ApplyOptions struct {
	DryRun  bool
	Repo    string
	Version string
}

func (h *Apply) ApplyChart(ctx context.Context,
	releaseName, releaseNamespace string,
	chartNameOrPath string, values map[string]interface{},
	options ApplyOptions,
) (*release.Release, error) {
	log := logr.FromContextOrDiscard(ctx).WithValues("name", releaseName, "namespace", releaseNamespace)

	if chartNameOrPath == "" {
		chartNameOrPath = releaseName
	}

	log.Info("loading chart")
	_, chart, err := LoadChart(ctx, chartNameOrPath, options.Repo, options.Version)
	if err != nil {
		return nil, err
	}
	if releaseName == "" {
		releaseName = chart.Name()
	}

	// do dry run
	if options.DryRun {
		log.Info("dry run installing")
		install := action.NewInstall(&action.Configuration{})
		install.ReleaseName, install.Namespace = releaseName, releaseNamespace
		install.DryRun, install.DisableHooks, install.ClientOnly, install.CreateNamespace = true, true, true, true
		return install.Run(chart, values)
	}

	cfg, err := NewHelmConfig(ctx, releaseNamespace, h.Config)
	if err != nil {
		return nil, err
	}
	existRelease, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if !errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, err
		}
		log.Info("installing", "values", values)
		install := action.NewInstall(cfg)
		install.ReleaseName, install.Namespace = releaseName, releaseNamespace
		install.CreateNamespace = true
		install.ClientOnly = options.DryRun
		return install.RunWithContext(ctx, chart, values)
	}
	// check should upgrade
	if existRelease.Info.Status == release.StatusDeployed && utils.EqualMapValues(existRelease.Config, values) {
		log.Info("already uptodate", "values", values)
		return existRelease, nil
	}
	log.Info("upgrading", "old", existRelease.Config, "new", values)
	client := action.NewUpgrade(cfg)
	client.Namespace = releaseNamespace
	client.ResetValues = true
	client.DryRun = options.DryRun

	// client.MaxHistory = 10 // there is a bug,do not use it.
	const historiesLimit = 2
	removeHistories(ctx, cfg.Releases, releaseName, historiesLimit)

	return client.RunWithContext(ctx, releaseName, chart, values)
}

func NewHelmConfig(ctx context.Context, namespace string, cfg *rest.Config) (*action.Configuration, error) {
	baselog := logr.FromContextOrDiscard(ctx)
	logfunc := func(format string, v ...interface{}) {
		baselog.Info(fmt.Sprintf(format, v...))
	}

	cligetter := genericclioptions.NewConfigFlags(true)
	cligetter.WrapConfigFn = func(*rest.Config) *rest.Config {
		return cfg
	}

	config := &action.Configuration{}
	config.Init(cligetter, namespace, "", logfunc) // release storage namespace
	if kc, ok := config.KubeClient.(*kube.Client); ok {
		kc.Namespace = namespace // install to namespace
	}
	return config, nil
}

// name is the name of the chart
// repo is the url of the chart repository,eg: http://charts.example.com
// if repopath is not empty,download it from repo and set chartNameOrPath to repo/repopath.
// LoadChart loads the chart from the repository
func LoadChart(ctx context.Context, nameOrPath, repo, version string) (string, *chart.Chart, error) {
	chartPathOptions := action.ChartPathOptions{RepoURL: repo, Version: version}
	settings := cli.New()
	chartPath, err := chartPathOptions.LocateChart(nameOrPath, settings)
	if err != nil {
		return "", nil, err
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		return "", nil, err
	}
	// dependencies update
	if err := action.CheckDependencies(chart, chart.Metadata.Dependencies); err != nil {
		man := &downloader.Manager{
			Out:              log.Default().Writer(),
			ChartPath:        chartPath,
			Keyring:          chartPathOptions.Keyring,
			SkipUpdate:       false,
			Getters:          getter.All(settings),
			RepositoryConfig: settings.RepositoryConfig,
			RepositoryCache:  settings.RepositoryCache,
			Debug:            settings.Debug,
		}
		if err := man.Update(); err != nil {
			return "", nil, err
		}
		chart, err = loader.Load(chartPath)
		if err != nil {
			return "", nil, err
		}
	}
	return chartPath, chart, nil
}

type RemoveOptions struct {
	DryRun bool
}

func (h *Apply) RemoveChart(ctx context.Context, releaseName, releaseNamespace string, options RemoveOptions) (*release.Release, error) {
	log := logr.FromContextOrDiscard(ctx)
	cfg, err := NewHelmConfig(ctx, releaseNamespace, h.Config)
	if err != nil {
		return nil, err
	}
	exist, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if !errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, err
		}
		return nil, nil
	}
	log.Info("uninstalling")
	uninstall := action.NewUninstall(cfg)
	uninstalledRelease, err := uninstall.Run(exist.Name)
	if err != nil {
		return nil, err
	}
	return uninstalledRelease.Release, nil
}

func removeHistories(ctx context.Context, storage *storage.Storage, name string, max int) error {
	rlss, err := storage.History(name)
	if err != nil {
		return err
	}
	if max <= 0 {
		max = 1
	}

	// newest to old
	slices.SortFunc(rlss, func(a, b *release.Release) bool {
		return a.Version > b.Version
	})

	var lastDeployed *release.Release
	toDelete := []*release.Release{}
	for _, rls := range rlss {
		if rls.Info.Status == release.StatusDeployed && lastDeployed == nil {
			lastDeployed = rls
			continue
		}
		// once we have enough releases to delete to reach the max, stop
		// all - deleted = max
		if len(rlss)-len(toDelete) == max {
			break
		}
		toDelete = append(toDelete, rls)
	}
	for _, todel := range toDelete {
		if _, err := storage.Delete(todel.Name, todel.Version); err != nil {
			return err
		}
	}
	return nil
}
