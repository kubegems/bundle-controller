# bundle controller

A controller manage helm charts and kustomize in kubernetes operator way.

## Features

- [x] basic helm installation management,install upgrade and uninstall.
- [x] kustomize management,render kustomize files and apply to kubernetes.
- [x] remote file support, download bundle from remote server.
  - [x] helm repository.
  - [x] git release tarball or other remote tarball file.
  - [x] git clone.
- [x] dependency check among bundles.
- [ ] helm charts version update check.

## Installation

Install bundle controller

```sh
kubectl create namespace bundle-controller
kubectl apply -f https://raw.githubusercontent.com/kubegems/bundle-controller/main/install.yaml
```

## Helm charts

Install a helm chart

```sh
cat <<EOF | kubectl apply -f -
apiVersion: bundle.kubegems.io/v1beta1
kind: Bundle
metadata:
  name: my-nginx # helm release name
spec:
  kind: helm
  chart: nginx # helm chart name
  url: https://charts.bitnami.com/bitnami
  version: 10.2.1
  values: # helm chart values
    ingress:
      enabled: true
EOF
```

Check the status of the helm bundle

```sh
$ kubectl get bundle
NAME       STATUS      NAMESPACE   VERSION   UPGRADETIMESTAMP   AGE
my-nginx   Installed   default     10.2.1    2s                 2s
```

Check the status of the helm release

```sh
$ helm list 
NAME            NAMESPACE       REVISION        UPDATED                                 STATUS          CHART           APP VERSION
my-nginx        default         1               2022-05-30 15:12:09.218912438 +0800 CST deployed        nginx-10.2.1    1.21.6     
```

For more helm usage, visit [docs/helm.md](docs/helm.md)

## Kustomize

Install a remote kustomize bundle from a git release tarball

```sh
cat <<EOF | kubectl apply -f -
apiVersion: bundle.kubegems.io/v1beta1
kind: Bundle
metadata:
  name: external-snapshotter
spec:
  kind: kustomize
  url: https://github.com/kubernetes-csi/external-snapshotter/archive/refs/tags/v5.0.1.tar.gz
  path: external-snapshotter-5.0.1/client/config/crd
EOF
```

Check the status of the kustomize bundle

```sh
$ kubectl get bundles.bundle.kubegems.io                        
NAME                   STATUS      NAMESPACE   VERSION   UPGRADETIMESTAMP   AGE
external-snapshotter   Installed   default               3s                 3s

$ kubectl get crd  | grep snapshot.storage.k8s.io
volumesnapshotclasses.snapshot.storage.k8s.io    2022-05-30T07:55:25Z
volumesnapshotcontents.snapshot.storage.k8s.io   2022-05-30T07:55:25Z
volumesnapshots.snapshot.storage.k8s.io          2022-05-30T07:55:25Z
```

For more kustomize usage, visit [docs/kustomize.md](docs/kustomize.md)

## Examples

For more examples, please visit [examples](examples).

## License

[License](License)
