# EKS Data Gatherer

The EKS *data gatherer* fetches information about a cluster from the AWS
Elastic Kubernetes Service API.

## Data

Preflight collects data about clusters. The fields included here can be found
[here](https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html).

## Configuration

To use the EKS data gatherer add an `eks` entry to the `data-gatherers`
configuration. For example:

```
data-gatherers:
- kind: "eks"
  name: "eks"
  config:
    cluster-name: my-eks-cluster
```

The `eks` configuration contains the following fields:

- `cluster-name`: The name of your EKS cluster.

## Permissions

Example Policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters"
      ],
      "Resource": "arn:aws:eks:*:111122223333:cluster/*"
    }
  ]
}
```
