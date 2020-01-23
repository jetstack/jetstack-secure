# Preflight Configuration

Configuration is provided to the Preflight application using a YAML file.
This specifies what packages to use, how data gatherers are configured,
and what outputs to produce.

Several example configuration files can be found in [`examples`](./examples).

## Cluster Name

The `cluster-name` field is used as the 'directory' prefix for output.
The value shouldn't contain spaces or `/`.
For example:

```
cluster-name: my-cluster
```

## Data Gatherers

Data gatherers are specified under the `data-gatherers` field.
For example:

```
data-gatherers:
  gke:
    project: my-gcp-project
    location: us-central1-a
    cluster: my-cluster
    credentials: /tmp/credentials.json
  k8s/pods:
    kubeconfig: ~/.kube/config
```

Each data gatherer has it's own configuration requirements,
which are documented separately.

The following data gatherers are available:

- [Kubernetes Pods](docs/datagatherers/k8s_pods.md)
- [Google Kubernetes Engine](docs/datagatherers/gke.md)
- [Amazon Elastic Kubernetes Service](docs/datagatherers/eks.md)
- [Microsoft Azure Kubernetes Service](docs/datagatherers/aks.md)

# Package Sources

The `package-sources` field is a list of locations
which Preflight should load packages from.
For example:

```
package-sources:
- type: local
  dir: ./preflight-packages/
- type: local
  dir: /home/user/other-preflight-packages
```

Each source must a `type`, though currently the only valid type is `local`.
Local sources must then specify a directory
for Preflight to look for packages in using the `dir` field.
Preflight will search for packages in this directory recursively.

In future other source types may be added,
for example to load packages in GCS buckets.

# Enabled Packages

The `enabled-packages` field is a list of packages that Preflight should use.
For example:

```
enabled-packages:
  - "examples.jetstack.io/gke_basic"
  - "jetstack.io/pods"
```

This allows `package-sources` to be large collections of packages,
only some of which will be run depending on user configuration.

## Outputs

The `outputs` field is a list of output formats and locations that Preflight
will write data to. Multiple outputs can be specified,
each with their own settings.

```
outputs:
- type: local
  path: ./output
  format: json
- type: local
  path: ./output
  format: intermediate
- type: cli
```

Possible types of output include:
- `local` for a local file.
- `gcs` for a Google Cloud Storage bucket.
- `cli` for command line output.

Most types also require a `format` to be specified.
Possible formats are:
- `json` for raw JSON output.
- `markdown` for a markdown formatted report.
- `html` for a HTML formatted report.
- `intermediate` to output the raw JSON fetched by the *data gatherers*.

With the `cli` type output the format is optional
and defaults to the `cli` format, for a coloured CLI formatted report.

The reports in `markdown`, `html` and `cli` format make use of the
*policy manifest* to produce a human readable report describing
 which checks passed and which failed.
The `json` format is raw output from OPA evaluation.

If no `outputs` are specified Preflight will output a report
of the results to the CLI.
