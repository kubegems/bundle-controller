# kustomize bundle

## Install

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

> The `.spec.path` is the path in the tarball to the kustomize directory.

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

To Install from a certain git reversion:

```diff
spec:
  kind: kustomize
--  url: https://github.com/kubernetes-csi/external-snapshotter/archive/refs/tags/v5.0.1.tar.gz
++  url: https://github.com/kubernetes-csi/external-snapshotter.git
++  version: v5.0.1
  path: external-snapshotter-5.0.1/client/config/crd
```

> The `.spec.version` is git revision name(tag\branch\commit hash).

## Remove

To remove a bundle, use the `kubectl delete` command.

```sh
kubectl delete bundle external-snapshotter
```
