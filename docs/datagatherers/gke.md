# GKE Data Gatherer

The GKE *data gatherer* fetches information about a cluster from the Google
Kubernetes Engine API.

## Data

The output of the GKE data gatherer follows the format described in the
[GKE API reference](https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1beta1/projects.locations.clusters#Cluster)
and the [Go Docs](https://godoc.org/google.golang.org/api/container/v1#Cluster).

It's comparable to the output from:

```bash
gcloud container clusters describe my-cluster --format=json
```

## Configuration

To use the GKE data gatherer add a `gke` entry to the `data-gatherers`
configuration. For example:

```
data-gatherers:
- kind: "gke"
  name: "gke"
  config:
    project: my-gcp-project
    location: us-central1-a
    cluster: my-gke-cluster
    # Path to a file containing the credentials. If empty, it will try to use
    # the SDK defaults
    # credentials: /tmp/credentials.json
```

The `gke` configuration contains the following fields:

- `project`: The ID of your Google Cloud Platform project.
- `location`: The compute zone or region where your cluster is running.
- `cluster`: The name of your GKE cluster.
- `credentials`: *optional* The path to a file containing credentials for your
  cluster.

## Permissions

If a `credentials` file is not specified, Preflight will attempt to use
Application Default Credentials or the metadata API (as per Google SDK default).

If Preflight is running locally and the `gcloud` command is installed and
configured, just run `gcloud auth application-default login` to set up
Application Default Credentials.

The `credentials` file path is useful if you want to configure a separate
service account for Preflight to use to fetch GKE data.

The user and service account must have the correct [IAM
Roles](https://cloud.google.com/kubernetes-engine/docs/how-to/iam).
Specifically it must have the `container.clusters.get` permission. This can be
given with the _Kubernetes Engine Cluster Viewer_ role
(`roles/container.clusterViewer`).

A sample Terraform project can be found at
[`./deployment/terraform/gke-datagatherer/`](deployment/terraform/gke-datagatherer).
This can be used to create a GCP service account called `preflight` which
is then bound to a custom role of the same name
with the minimum required permissions.
