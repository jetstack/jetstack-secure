# AKS Data Gatherer

The AKS *data gatherer* fetches information about a cluster from the Azure
Kubernetes Service API.

## Data

Preflight collects data about clusters. The fields included here can be found
[here](https://docs.microsoft.com/en-us/rest/api/aks/managedclusters/get).

## Configuration

To use the AKS data gatherer add an `aks` entry to the `data-gatherers`
configuration. For example:

```
data-gatherers:
- kind: "aks"
  name: "aks"
  config:
    resource-group: example
    cluster-name: my-aks-cluster
    credentials-path: /tmp/credentials.json
```

The `aks` configuration contains the following fields:

- `resource-group`: The Azure resource group where the cluster is located.
- `cluster-name`: The name of your AKS cluster.
- `credentials`: The path to a file containing credentials for Azure APIs.

## Permissions

You must [create](https://docs.microsoft.com/en-us/azure/aks/kubernetes-service-principal#manually-create-a-service-principal)
a Service Principal and [link](https://docs.microsoft.com/en-us/azure/aks/kubernetes-service-principal#specify-a-service-principal-for-an-aks-cluster)
a it to your AKS cluster.
