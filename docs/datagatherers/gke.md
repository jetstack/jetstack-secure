# GKE data gatherer

It pulls information about one cluster from the GKE API.

## Configuration

[Here](../../examples/gke.preflight.yaml) you have a sample configuration file setting up the GKE data gatherer.

You have to set these parameters in the configuration:

- **project:** the ID of your Google Cloud Platform project.
- **location:** the compute zone or region where your cluster is running.
- **cluster:** the name of your GKE cluster.
- **credentials** *optional* **:** path to a file containing valid credentials for your cluster. Useful if you want to configure a separate service account. If not specified, it will attept to use Workload Identity. If you run Preflight locally on your machine, you can just run `gcloud auth application-default login`

## Data

The output of the GKE data gatherer follows this format:

```json
{
  "Cluster": {...}
}
```

The `Cluster` property is a JSON representation of [google.golang.org/api/container/v1#Cluster](https://godoc.org/google.golang.org/api/container/v1#Cluster).

> Tip: Use the 'intermediate' output format to get the raw output from the data gatherer. You can use that try your rego rules.
