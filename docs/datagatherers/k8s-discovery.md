# k8s-discovery

This datagatherer uses the [DiscoveryClient](https://godoc.org/k8s.io/client-go/discovery#DiscoveryClient)
to get API server version information.

Include the following in your agent config:

```
data-gatherers:
- kind: "k8s-discovery"
  name: "k8s-discovery"
```

or specify a kubeconfig file:

```
data-gatherers:
- kind: "k8s-discovery"
  name: "k8s-discovery"
  config:
    kubeconfig: other_kube_config_path
```
