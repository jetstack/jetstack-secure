# Kubernetes Pods Data Gatherer

The Kubernetes Pods *data gatherer* gets information about
the Pods running in a cluster.

## Data

The output of the k8s/pods data gatherer is a JSON representation of a
[PodList](https://godoc.org/k8s.io/api/core/v1#PodList).

## Configuration

To use the Kubernetes Pods *data gatherer* add a `k8s/pods` section to the
`data-gatherers` section in your configuration like so:

```
data-gatherers:
  k8s/pods:
    kubeconfig: ~/.kube/config
```

The `kubeconfig` field should point to your *kubeconfig* file.
This is typically found at `~/.kube/config`.
Preflight will use the context that is active in that *kubeconfig*.

An example configuration can be found at
[`./examples/pods.preflight.yaml`](./examples/pods.preflight.yaml).

# Permissions

The user or service account used by the *kubeconfig* to authenticate with 
the Kubernetes API must have permission to perform `list` and `get`
on `Pod` resources.

There is an example `ClusterRole` and `ClusterRoleBinding` which can be found in
[`./deployment/kubernetes/base/00-rbac.yaml`](./deployment/kubernetes/base/00-rbac.yaml).

If the cluster is on GKE the user or service account used to access the
Kubernetes API will require the `container.pods.get`
and `container.pods.list` IAM permissions.
These can be given with the _Kubernetes Engine Viewer_ role
(`roles/container.viewer`), which also includes the 
`container.clusters.get` permission required to use the
[GKE *data gatherer*](./docs/datagatherers/gke.md).
Alternatively a custom role can be defined with only the permissions named here.
