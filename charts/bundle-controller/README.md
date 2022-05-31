# bundle-controller

Helm chart for the bundle-controller.

## TL;DR

```console
helm install bundle-controller ./charts/bundle-controller
```

## Introduction

A controller manage helm charts and kustomize in kubernetes operator ways.

## Prerequisites

- Kubernetes 1.21+

## Installing the Chart

To install the chart:

```console
helm install bundle-controller ./charts/bundle-controller
```

The command deploys bundle-controller on the Kubernetes cluster in the default configuration.

The [Parameters](#parameters) section lists the parameters
that can be configured during installation.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```console
helm delete bundle-controller
```

The command removes all the Kubernetes components associated
with the chart and deletes the release.

## Parameters

### Global parameters

| Name                      | Description                                     | Value |
| ------------------------- | ----------------------------------------------- | ----- |
| `global.imageRegistry`    | Global Docker image registry                    | `""`  |
| `global.imagePullSecrets` | Global Docker registry secret names as an array | `[]`  |
| `global.storageClass`     | Global StorageClass for Persistent Volume(s)    | `""`  |

### Common parameters

| Name                     | Description                                                                             | Value           |
| ------------------------ | --------------------------------------------------------------------------------------- | --------------- |
| `kubeVersion`            | Override Kubernetes version                                                             | `""`            |
| `nameOverride`           | String to partially override common.names.fullname                                      | `""`            |
| `fullnameOverride`       | String to fully override common.names.fullname                                          | `""`            |
| `commonLabels`           | Labels to add to all deployed objects                                                   | `{}`            |
| `commonAnnotations`      | Annotations to add to all deployed objects                                              | `{}`            |
| `clusterDomain`          | Kubernetes cluster domain name                                                          | `cluster.local` |
| `extraDeploy`            | Array of extra objects to deploy with the release                                       | `[]`            |
| `diagnosticMode.enabled` | Enable diagnostic mode (all probes will be disabled and the command will be overridden) | `false`         |
| `diagnosticMode.command` | Command to override all containers in the deployment                                    | `["sleep"]`     |
| `diagnosticMode.args`    | Args to override all containers in the deployment                                       | `["infinity"]`  |

### bundle Parameters

| Name                                           | Description                                                                                      | Value                        |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------- |
| `bundle.image.registry`                        | bundle image registry                                                                            | `docker.io`                  |
| `bundle.image.repository`                      | bundle image repository                                                                          | `kubegems/bundle-controller` |
| `bundle.image.tag`                             | bundle image tag (immutable tags are recommended)                                                | `latest`                     |
| `bundle.image.pullPolicy`                      | bundle image pull policy                                                                         | `IfNotPresent`               |
| `bundle.image.pullSecrets`                     | bundle image pull secrets                                                                        | `[]`                         |
| `bundle.image.debug`                           | Enable bundle image debug mode                                                                   | `false`                      |
| `bundle.replicaCount`                          | Number of bundle replicas to deploy                                                              | `1`                          |
| `bundle.containerPorts.probe`                  | bundle probe container port                                                                      | `8080`                       |
| `bundle.livenessProbe.enabled`                 | Enable livenessProbe on bundle containers                                                        | `true`                       |
| `bundle.livenessProbe.initialDelaySeconds`     | Initial delay seconds for livenessProbe                                                          | `5`                          |
| `bundle.livenessProbe.periodSeconds`           | Period seconds for livenessProbe                                                                 | `10`                         |
| `bundle.livenessProbe.timeoutSeconds`          | Timeout seconds for livenessProbe                                                                | `5`                          |
| `bundle.livenessProbe.failureThreshold`        | Failure threshold for livenessProbe                                                              | `6`                          |
| `bundle.livenessProbe.successThreshold`        | Success threshold for livenessProbe                                                              | `1`                          |
| `bundle.readinessProbe.enabled`                | Enable readinessProbe on bundle containers                                                       | `true`                       |
| `bundle.readinessProbe.initialDelaySeconds`    | Initial delay seconds for readinessProbe                                                         | `5`                          |
| `bundle.readinessProbe.periodSeconds`          | Period seconds for readinessProbe                                                                | `10`                         |
| `bundle.readinessProbe.timeoutSeconds`         | Timeout seconds for readinessProbe                                                               | `5`                          |
| `bundle.readinessProbe.failureThreshold`       | Failure threshold for readinessProbe                                                             | `6`                          |
| `bundle.readinessProbe.successThreshold`       | Success threshold for readinessProbe                                                             | `1`                          |
| `bundle.startupProbe.enabled`                  | Enable startupProbe on bundle containers                                                         | `false`                      |
| `bundle.startupProbe.initialDelaySeconds`      | Initial delay seconds for startupProbe                                                           | `5`                          |
| `bundle.startupProbe.periodSeconds`            | Period seconds for startupProbe                                                                  | `10`                         |
| `bundle.startupProbe.timeoutSeconds`           | Timeout seconds for startupProbe                                                                 | `5`                          |
| `bundle.startupProbe.failureThreshold`         | Failure threshold for startupProbe                                                               | `6`                          |
| `bundle.startupProbe.successThreshold`         | Success threshold for startupProbe                                                               | `1`                          |
| `bundle.customLivenessProbe`                   | Custom livenessProbe that overrides the default one                                              | `{}`                         |
| `bundle.customReadinessProbe`                  | Custom readinessProbe that overrides the default one                                             | `{}`                         |
| `bundle.customStartupProbe`                    | Custom startupProbe that overrides the default one                                               | `{}`                         |
| `bundle.resources.limits`                      | The resources limits for the bundle containers                                                   | `{}`                         |
| `bundle.resources.requests`                    | The requested resources for the bundle containers                                                | `{}`                         |
| `bundle.podSecurityContext.enabled`            | Enabled bundle pods' Security Context                                                            | `false`                      |
| `bundle.podSecurityContext.fsGroup`            | Set bundle pod's Security Context fsGroup                                                        | `1001`                       |
| `bundle.containerSecurityContext.enabled`      | Enabled bundle containers' Security Context                                                      | `false`                      |
| `bundle.containerSecurityContext.runAsUser`    | Set bundle containers' Security Context runAsUser                                                | `1001`                       |
| `bundle.containerSecurityContext.runAsNonRoot` | Set bundle containers' Security Context runAsNonRoot                                             | `true`                       |
| `bundle.leaderElection.enabled`                | Enable leader election                                                                           | `true`                       |
| `bundle.logLevel`                              | Log level                                                                                        | `debug`                      |
| `bundle.existingConfigmap`                     | The name of an existing ConfigMap with your custom configuration for bundle                      | `""`                         |
| `bundle.command`                               | Override default container command (useful when using custom images)                             | `[]`                         |
| `bundle.args`                                  | Override default container args (useful when using custom images)                                | `[]`                         |
| `bundle.hostAliases`                           | bundle pods host aliases                                                                         | `[]`                         |
| `bundle.podLabels`                             | Extra labels for bundle pods                                                                     | `{}`                         |
| `bundle.podAnnotations`                        | Annotations for bundle pods                                                                      | `{}`                         |
| `bundle.podAffinityPreset`                     | Pod affinity preset. Ignored if `bundle.affinity` is set. Allowed values: `soft` or `hard`       | `""`                         |
| `bundle.podAntiAffinityPreset`                 | Pod anti-affinity preset. Ignored if `bundle.affinity` is set. Allowed values: `soft` or `hard`  | `soft`                       |
| `bundle.nodeAffinityPreset.type`               | Node affinity preset type. Ignored if `bundle.affinity` is set. Allowed values: `soft` or `hard` | `""`                         |
| `bundle.nodeAffinityPreset.key`                | Node label key to match. Ignored if `bundle.affinity` is set                                     | `""`                         |
| `bundle.nodeAffinityPreset.values`             | Node label values to match. Ignored if `bundle.affinity` is set                                  | `[]`                         |
| `bundle.enableAffinity`                        | If enabled Affinity for bundle pods assignment                                                   | `false`                      |
| `bundle.affinity`                              | Affinity for bundle pods assignment                                                              | `{}`                         |
| `bundle.nodeSelector`                          | Node labels for bundle pods assignment                                                           | `{}`                         |
| `bundle.tolerations`                           | Tolerations for bundle pods assignment                                                           | `[]`                         |
| `bundle.updateStrategy.type`                   | bundle statefulset strategy type                                                                 | `RollingUpdate`              |
| `bundle.priorityClassName`                     | bundle pods' priorityClassName                                                                   | `""`                         |
| `bundle.schedulerName`                         | Name of the k8s scheduler (other than default) for bundle pods                                   | `""`                         |
| `bundle.lifecycleHooks`                        | for the bundle container(s) to automate configuration before or after startup                    | `{}`                         |
| `bundle.extraEnvVars`                          | Array with extra environment variables to add to bundle nodes                                    | `[]`                         |
| `bundle.extraEnvVarsCM`                        | Name of existing ConfigMap containing extra env vars for bundle nodes                            | `[]`                         |
| `bundle.extraEnvVarsSecret`                    | Name of existing Secret containing extra env vars for bundle nodes                               | `[]`                         |
| `bundle.extraVolumes`                          | Optionally specify extra list of additional volumes for the bundle pod(s)                        | `[]`                         |
| `bundle.extraVolumeMounts`                     | Optionally specify extra list of additional volumeMounts for the bundle container(s)             | `[]`                         |
| `bundle.sidecars`                              | Add additional sidecar containers to the bundle pod(s)                                           | `{}`                         |
| `bundle.initContainers`                        | Add additional init containers to the bundle pod(s)                                              | `{}`                         |

### Agent Metrics parameters

| Name                                              | Description                                                                 | Value                    |
| ------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------ |
| `bundle.metrics.enabled`                          | Create a service for accessing the metrics endpoint                         | `true`                   |
| `bundle.metrics.service.type`                     | controller metrics service type                                             | `ClusterIP`              |
| `bundle.metrics.service.port`                     | controller metrics service HTTP port                                        | `9100`                   |
| `bundle.metrics.service.nodePort`                 | Node port for HTTP                                                          | `""`                     |
| `bundle.metrics.service.clusterIP`                | controller metrics service Cluster IP                                       | `""`                     |
| `bundle.metrics.service.extraPorts`               | Extra ports to expose (normally used with the `sidecar` value)              | `[]`                     |
| `bundle.metrics.service.loadBalancerIP`           | controller metrics service Load Balancer IP                                 | `""`                     |
| `bundle.metrics.service.loadBalancerSourceRanges` | controller metrics service Load Balancer sources                            | `[]`                     |
| `bundle.metrics.service.externalTrafficPolicy`    | controller metrics service external traffic policy                          | `Cluster`                |
| `bundle.metrics.service.annotations`              | Additional custom annotations for controller metrics service                | `{}`                     |
| `bundle.metrics.serviceMonitor.enabled`           | Specify if a servicemonitor will be deployed for prometheus-operator        | `true`                   |
| `bundle.metrics.serviceMonitor.jobLabel`          | Specify the jobLabel to use for the prometheus-operator                     | `app.kubernetes.io/name` |
| `bundle.metrics.serviceMonitor.honorLabels`       | Honor metrics labels                                                        | `false`                  |
| `bundle.metrics.serviceMonitor.selector`          | Prometheus instance selector labels                                         | `{}`                     |
| `bundle.metrics.serviceMonitor.scrapeTimeout`     | Timeout after which the scrape is ended                                     | `""`                     |
| `bundle.metrics.serviceMonitor.interval`          | Scrape interval. If not set, the Prometheus default scrape interval is used | `""`                     |
| `bundle.metrics.serviceMonitor.additionalLabels`  | Used to pass Labels that are required by the installed Prometheus Operator  | `{}`                     |
| `bundle.metrics.serviceMonitor.metricRelabelings` | Specify additional relabeling of metrics                                    | `{}`                     |
| `bundle.metrics.serviceMonitor.relabelings`       | Specify general relabeling                                                  | `{}`                     |

### RBAC Parameters

| Name                    | Description                                                   | Value  |
| ----------------------- | ------------------------------------------------------------- | ------ |
| `rbac.create`           | Specifies whether RBAC resources should be created            | `true` |
| `rbac.useClusterAdmin`  | clusterrolbinding to cluster-admin instead create clusterrole | `true` |
| `serviceAccount.create` | Specifies whether a ServiceAccount should be created          | `true` |
| `serviceAccount.name`   | The name of the ServiceAccount to use.                        | `""`   |
