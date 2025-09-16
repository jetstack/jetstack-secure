# Kubernetes Data Gatherer

The Kubernetes dynamic data gatherer collects information about resources stored
in the Kubernetes API.

## Data

The data gathered depends on your configuration. Resources are selected based on
their Group-Version-Kind identifiers, e.g.:

* Core resources such as `Service`, use: `k8s/services.v1`
* `Ingress`, use: `k8s/ingresses.v1beta1.networking.k8s.io`
* Custom resources such as `Certificates`, use:
  `k8s/certificates.v1alpha2.cert-manager.io`

To see an example of the data being gathered, using `k8s/services.v1` is
comparable to the output from:

```bash
kubectl get services --all-namespaces -o json
```

## Configuration

You can collect different resources using difference Group-Version-Kind as
below:

```yaml
data-gatherers:
# basic usage
- kind: "k8s-dynamic"
  name: "k8s/pods"
  config:
    resource-type:
      resource: pods
      version: v1

# CRD usage
- kind: "k8s-dynamic"
  name: "k8s/certificates.v1alpha2.cert-manager.io"
  config:
    resource-type:
      group: cert-manager.io
      version: v1alpha2
      resource: certificates

# you might event want to gather resources from another cluster
- kind: "k8s-dynamic"
  name: "k8s/pods"
  config:
    kubeconfig: other_kube_config_path
```

The `kubeconfig` field should point to your Kubernetes config file - this is
typically found at `~/.kube/config`. Preflight will use the context that is
active in that config file.

## Permissions

The user or service account used by the Kubernetes config to authenticate with
the Kubernetes API must have permission to perform `list` and `get` on the
resource referenced in the `kind` for that datagatherer.

There is an example `ClusterRole` and `ClusterRoleBinding` which can be found in
[`./deployment/kubernetes/base/00-rbac.yaml`](./deployment/kubernetes/base/00-rbac.yaml).

## Secrets

Secrets can be gathered using the following config:

```yaml
- kind: "k8s-dynamic"
  name: "k8s/secrets"
  config:
    resource-type:
      version: v1
      resource: secrets
```

Before Secrets are sent to the Preflight backend, they are redacted so no secret data is transmitted. See [`fieldfilter.go`](./../../pkg/datagatherer/k8s/fieldfilter.go) to see the details of which fields are filteres and which ones are redacted.

> **All resource other than Kubernetes Secrets are sent in full, so make sure that you don't store secret information on arbitrary resources.**


## Field Selectors

You can use [field selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/#list-of-supported-fields)
to include or exclude certain resources.
For example, you can reduce the memory usage of the agent and reduce the load on the Kubernetes
API server by omitting various common [Secret types](https://kubernetes.io/docs/concepts/configuration/secret/#secret-types)
when listing Secrets.

```yaml
- kind: "k8s-dynamic"
  name: "k8s/secrets"
  config:
    resource-type:
      version: v1
      resource: secrets
    field-selectors:
    - type!=kubernetes.io/service-account-token
    - type!=kubernetes.io/dockercfg
    - type!=kubernetes.io/dockerconfigjson
    - type!=kubernetes.io/basic-auth
    - type!=kubernetes.io/ssh-auth,
    - type!=bootstrap.kubernetes.io/token
    - type!=helm.sh/release.v1
```
