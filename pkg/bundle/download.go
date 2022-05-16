package bundle

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-logr/logr"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/helm"
)

const (
	defaultDirMode  = 0o755
	defaultFileMode = 0o644
)

// we cache "plugin" in a directory with name "{name}-{version}" under cache directory
func DownloadPlugin(ctx context.Context, plugin *Plugin, cachedir string, searchdirs ...string) error {
	log := logr.FromContextOrDiscard(ctx).WithValues("name", plugin.Name, "version", plugin.Version)
	// from search path and cache
	if plugin.Version == "" {
		plugin.Version = "latest"
	}
	if cachedir == "" {
		cachedir = ".cache"
	}
	pluginpath := fmt.Sprintf("%s-%s", plugin.Name, plugin.Version)
	for _, dir := range append(searchdirs, cachedir) {
		// try without version
		fullsearchpath := filepath.Join(dir, pluginpath)
		if entries, err := os.ReadDir(fullsearchpath); err == nil && len(entries) > 0 {
			log.Info("found in search path", "dir", pluginpath)
			plugin.DownloadTo = fullsearchpath
			return nil
		}
	}
	plugincachepath := filepath.Join(cachedir, pluginpath)
	log.Info("downloading...", "cache", plugincachepath)
	if err := Download(ctx, plugin.Source, plugincachepath); err != nil {
		log.Error(err, "on download", "cache", plugincachepath)
		return err
	}
	log.Info("download", "cache", plugincachepath)
	plugin.DownloadTo = plugincachepath
	return nil
}

// cases
// 1. URI: charts.example.com/repository
// 1. URI: files.example.com/blob/filename.tgz
// 1. URI: git.example.com/foo/bar.git														Subpath: deploy/manifests
// 1. URI: https://github.com/rancher/local-path-provisioner/archive/refs/tags/v0.0.22.zip	Subpath: deploy/manifests
// 1. URI: https://github.com/rancher/local-path-provisioner/archive/refs/heads/master.zip 	Subpath:

type DownloadSource struct {
	Helm *bundlev1.HelmSource
	Git  *bundlev1.GitSource
	S3   *bundlev1.S3Source
	Http *bundlev1.HttpSource
}

func Download(ctx context.Context, source DownloadSource, intodir string) error {
	if source.Helm != nil {
		return DownloadHelmChart(ctx, source.Helm.URL, source.Helm.Name, source.Helm.Version, intodir)
	}
	if source.Git != nil {
		return DownloadGit(ctx, source.Git.URL, source.Git.Revision, source.Git.Path, intodir)
	}
	if source.S3 != nil {
		return DownloadS3(ctx, source.S3.URL, source.S3.Path, intodir)
	}
	if source.Http != nil {
		ext := filepath.Ext(path.Base(source.Http.URL))
		switch ext {
		case ".tgz", ".tar.gz", ".gz":
			return DownloadTgz(ctx, source.Http.URL, source.Http.Path, intodir)
		case ".zip":
			return DownloadZip(ctx, source.Http.URL, source.Http.Path, intodir)
		default:
			return fmt.Errorf("unsupported http file type %s", path.Base(source.Http.URL))
		}
	}
	return fmt.Errorf("unknown download source")
}

func DownloadS3(ctx context.Context, url string, path string, intodir string) error {
	return nil
}

func DownloadZip(ctx context.Context, uri, subpath, into string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	r := bytes.NewReader(raw)
	zipr, err := zip.NewReader(r, r.Size())
	if err != nil {
		return err
	}

	if subpath != "" && !strings.HasSuffix(subpath, "/") {
		subpath += "/"
	}

	for _, file := range zipr.File {
		if !strings.HasPrefix(file.Name, subpath) {
			continue
		}
		{
			filename := strings.TrimPrefix(file.Name, subpath)
			filename = filepath.Join(into, filename)

			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(filename, file.Mode()); err != nil {
					return err
				}
				continue
			}

			dest, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer dest.Close()

			src, err := file.Open()
			if err != nil {
				return err
			}
			defer src.Close()
			_, _ = io.Copy(dest, src)
		}
	}
	return nil
}

func DownloadTgz(ctx context.Context, uri, subpath, into string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return UnTarGz(resp.Body, subpath, into)
}

func DownloadFile(ctx context.Context, src string, subpath, into string) error {
	u, err := url.ParseRequestURI(src)
	if err != nil {
		return err
	}
	if u.Host != "" && u.Host != "localhost" {
		return fmt.Errorf("unsupported host: %s", u.Host)
	}

	basedir := u.Path
	if !strings.HasSuffix(basedir, "/") {
		basedir += "/"
	}

	if err := os.MkdirAll(into, defaultDirMode); err != nil {
		return err
	}

	return filepath.WalkDir(basedir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relpath := strings.TrimPrefix(path, basedir)

		if !strings.HasPrefix(relpath, subpath) {
			return nil
		}

		filename := strings.TrimPrefix(relpath, subpath)
		filename = filepath.Join(into, filename)

		fi, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := os.MkdirAll(filename, fi.Mode().Perm()); err != nil {
				return err
			}
			return nil
		}
		dest, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
		if err != nil {
			return err
		}
		defer dest.Close()

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, _ = io.Copy(dest, f)
		return nil
	})
}

func DownloadGit(ctx context.Context, cloneurl string, rev string, subpath, into string) error {
	repository, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
		URL:          cloneurl,
		Depth:        1,
		SingleBranch: true,
	})
	if err != nil {
		return err
	}

	if rev == "" {
		rev = "HEAD"
	}
	hash, err := repository.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return err
	}

	commit, err := repository.CommitObject(*hash)
	if err != nil {
		return err
	}

	tree, err := repository.TreeObject(commit.TreeHash)
	if err != nil {
		return err
	}

	return tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasPrefix(f.Name, subpath) {
			return nil
		}
		raw, err := f.Contents()
		if err != nil {
			return err
		}

		fmode, err := f.Mode.ToOSFileMode()
		if err != nil {
			fmode = defaultFileMode
		}

		filename := strings.TrimPrefix(f.Name, subpath)
		filename = filepath.Join(into, filename)
		if dir := filepath.Dir(filename); dir != "" {
			if err := os.MkdirAll(dir, defaultDirMode); err != nil {
				return err
			}
		}
		return os.WriteFile(filename, []byte(raw), fmode)
	})
}

func DownloadHelmChart(ctx context.Context, repo, name, version, intodir string) error {
	chartPath, chart, err := helm.LoadChart(ctx, name, repo, version)
	if err != nil {
		return err
	}
	// untgz chartPath into intodir
	f, err := os.Open(chartPath)
	if err != nil {
		return err
	}
	return UnTarGz(f, chart.Name(), intodir)
}

func UnTarGz(r io.Reader, subpath, into string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if !strings.HasPrefix(hdr.Name, subpath) {
			continue
		}

		filename := strings.TrimPrefix(hdr.Name, subpath)
		filename = filepath.Join(into, filename)

		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(filename, defaultDirMode); err != nil {
				return err
			}
			continue
		} else {
			if err := os.MkdirAll(filepath.Dir(filename), defaultDirMode); err != nil {
				return err
			}
		}

		dest, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		defer dest.Close()

		_, _ = io.Copy(dest, tr)
	}
	return nil
}