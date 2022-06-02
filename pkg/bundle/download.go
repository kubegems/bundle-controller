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
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/go-logr/logr"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
	"kubegems.io/bundle-controller/pkg/bundle/helm"
)

const (
	defaultDirMode  = 0o755
	defaultFileMode = 0o644
)

// we cache "bundle" in a directory with name "{name}-{version}" under cache directory
func Download(ctx context.Context, bundle *bundlev1.Bundle, cachedir string, searchdirs ...string) (string, error) {
	log := logr.FromContextOrDiscard(ctx)
	if cachedir == "" {
		home, _ := os.UserHomeDir()
		cachedir = filepath.Join(home, ".cache", "kubegems", "bundles")
	}

	name, version := getCacheNameVersion(bundle)

	// from searchdirs
	versionedPath, unVersionedPath := fmt.Sprintf("%s-%s", name, version), name
	for _, dir := range searchdirs {
		if foundpath := findAt(filepath.Join(dir, versionedPath)); foundpath != "" {
			log.Info("found in search path", "path", foundpath)
			return foundpath, nil
		}
		if foundpath := findAt(filepath.Join(dir, unVersionedPath)); foundpath != "" {
			log.Info("found in search path", "path", foundpath)
			return foundpath, nil
		}
	}

	// from cache
	fullVersionedPath := filepath.Join(cachedir, versionedPath)
	if foundpath := findAt(fullVersionedPath); foundpath != "" {
		log.Info("found in cache path", "path", foundpath)
		return foundpath, nil
	}

	repo := bundle.Spec.URL
	if repo == "" {
		// use path as local file path
		if path, ok := isNotEmpty(bundle.Spec.Path); ok {
			return path, nil
		}
		return "", fmt.Errorf("no find in search pathes and no download url specified")
	}

	into := fullVersionedPath
	log.Info("downloading...", "cache", into)

	// is file://
	if strings.HasPrefix(repo, "file://") {
		return into, DownloadFile(ctx, repo, bundle.Spec.Path, into)
	}
	// is git ?
	if strings.HasSuffix(repo, ".git") {
		return into, DownloadGit(ctx, repo, bundle.Spec.Version, bundle.Spec.Path, into)
	}
	// is zip ?
	if strings.HasSuffix(repo, ".zip") {
		return into, DownloadZip(ctx, repo, bundle.Spec.Path, into)
	}
	// is tar.gz ?
	if strings.HasSuffix(repo, ".tar.gz") || strings.HasSuffix(repo, ".tgz") {
		return into, DownloadTgz(ctx, repo, bundle.Spec.Path, into)
	}
	// is helm repo?
	if bundle.Spec.Kind == bundlev1.BundleKindHelm {
		return DownloadHelmChart(ctx, repo, name, version, into)
	}
	return "", fmt.Errorf("unknown download source")
}

func getCacheNameVersion(bundle *bundlev1.Bundle) (string, string) {
	version := bundle.Spec.Version
	if version == "" {
		version = "0.0.0"
	}
	name := bundle.Spec.Chart
	if name == "" {
		name = bundle.Name
	}
	return name, version
}

func DownloadS3(ctx context.Context, url string, bucket string, path string, intodir string) error {
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

func DownloadHelmChart(ctx context.Context, repo, name, version, intodir string) (string, error) {
	log := logr.FromContextOrDiscard(ctx)
	chartPath, _, err := helm.LoadChart(ctx, name, repo, version)
	if err != nil {
		return "", err
	}
	intofile := filepath.Join(filepath.Dir(intodir), fmt.Sprintf("%s.tgz", filepath.Base(intodir)))
	os.MkdirAll(filepath.Dir(intofile), defaultDirMode)
	log.Info("downloaded chart", "dir", intofile)
	// just move the chart.tgz into intodir
	return intofile, os.Rename(chartPath, intofile)
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

func findAt(path string) string {
	if tgzfile, ok := hasTgz(path); ok {
		return tgzfile
	}
	if cachedir, ok := isNotEmpty(path); ok {
		return cachedir
	}
	return ""
}

func isNotEmpty(path string) (string, bool) {
	entries, err := os.ReadDir(path)
	return path, (err == nil && len(entries) >= 0)
}

func hasTgz(path string) (string, bool) {
	for _, p := range []string{path + ".tgz", path + ".tar.gz"} {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return path, false
}
