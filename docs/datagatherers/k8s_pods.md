# k8s/pods data gatherer

It pulls information about the Pods running in a cluster.

## Configuration

[Here](../../examples/pods.preflight.yaml) you have a sample configuration file setting up the k8s/pods data gatherer.

You just have to set the `kubeconfig` parameter in the configuration. It should point to your kubeconfig* file.

Preflight will use the context that is active in that *kubeconfig*.

## Data

The output of the k8s/pods data gatherer is a JSON representation of a [PodList](https://godoc.org/k8s.io/api/core/v1#PodList).

> Tip: Use the 'intermediate' output format to get the raw output from the data gatherer. You can use that try your rego rules.
