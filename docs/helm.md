# helm bundle

manage helm installation with bundle controller.

## Install

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

Check the status of the bundle status:

```sh
$ kubectl get bundles.bundle.kubegems.io my-nginx -ojsonpath='{.status}' | yq -P

creationTimestamp: "2022-05-30T08:31:05Z"
message: |
  CHART NAME: nginx
  CHART VERSION: 10.2.1
  APP VERSION: 1.21.6

  ** Please be patient while the chart is being deployed **

  NGINX can be accessed through the following DNS name from within your cluster:

      my-nginx.default.svc.cluster.local (port 80)

  To access NGINX from outside the cluster, follow the steps below:

  1. Get the NGINX URL and associate its hostname to your cluster external IP:

     export CLUSTER_IP=$(minikube ip) # On Minikube. Use: `kubectl cluster-info` on others K8s clusters
     echo "NGINX URL: http://nginx.local"
     echo "$CLUSTER_IP  nginx.local" | sudo tee -a /etc/hosts
namespace: default
phase: Installed
resources:
  - apiVersion: v1
    kind: Service
    name: my-nginx
  - apiVersion: apps/v1
    kind: Deployment
    name: my-nginx
  - apiVersion: networking.k8s.io/v1
    kind: Ingress
    name: my-nginx
upgradeTimestamp: "2022-05-30T08:31:05Z"
values:
  ingress:
    enabled: true
version: 10.2.1
```

Check the status of the helm release:

```sh
$ helm list 
NAME            NAMESPACE       REVISION        UPDATED                                 STATUS          CHART           APP VERSION
my-nginx        default         1               2022-05-30 16:31:05.183080645 +0800 CST deployed        nginx-10.2.1    1.21.6     
```

You can also reference values from configmap:

```sh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: global-values
data:
  global.imageRegistry: "quay.io"
EOF

cat <<EOF | kubectl apply -f -
apiVersion: bundle.kubegems.io/v1beta1
kind: Bundle
metadata:
  name: my-nginx
spec:
  kind: helm
  chart: nginx
  url: https://charts.bitnami.com/bitnami
  version: 10.2.1
  valuesRef:
    - kind: ConfigMap
      name: global-values
  values:
    ingress:
      enabled: true
EOF
```

Check the values of the helm release:

```sh
$ helm get values my-nginx                                                   
USER-SUPPLIED VALUES:
global:
  imageRegistry: quay.io
ingress:
  enabled: true
```

To install helm to another namespace:

```diff
...
spec:
  kind: helm
  chart: nginx
++  namespace: my-namespace
  url: https://charts.bitnami.com/bitnami
  version: 10.2.1
...
```

## Upgrade

To Upgrade a helm release, just update the values:

```sh
kubectl patch bundles my-nginx  --patch='{"spec":{"values":{"service":{"type":"NodePort"}}}}' --type=merge
```

Check the status of the helm release:

```sh
$ helm get values my-nginx                                                                                                     
USER-SUPPLIED VALUES:
ingress:
  enabled: true
service:
  type: NodePort
```

## Uninstall

To uninstall a helm release, just delete the bundle:

```sh
kubectl delete bundle my-nginx
```

```sh
$ helm get all my-nginx         
Error: release: not found
```
