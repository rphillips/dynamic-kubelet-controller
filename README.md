# Dynamic Kubelet Controller

Kubernetes 1.11 migrates [Kubelet Dynamic
Configuration](https://kubernetes.io/docs/tasks/administer-cluster/reconfigure-kubelet/)
to Beta. This repository contains a controller to automatically add
ConfigMaps to Kubelets joining the cluster. Kubelet configuration through the
Kubernetes API is extremely useful to allow for central administration of
configs, and consistency of no one-off solutions for specific nodes within a
cluster.

## Basics

Static Mappings:

| Label                           | ConfigMap Name                     |
| ------------------------------- | ---------------------------------- |
| node-role.kubernetes.io/master  | openshift-node/node-config-master  |
| node-role.kubernetes.io/worker  | openshift-node/node-config-worker |

Custom ConfigMaps:

| Label                          | Value                         |
| ------------------------------ | ----------------------------- |
| node-role.kubernetes.io/custom | namespace/your-configmap-name |

## Compiling and Install

This project leverages [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) 1.0.

Depedencies:

* golang (1.10.3)
* [kustomize](https://github.com/kubernetes-sigs/kustomize) (tested with 1.0.4)

```shell
make
```

Deploying:

```shell
make deploy
```
