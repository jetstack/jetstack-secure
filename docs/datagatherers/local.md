# Local Data Gatherer

The Local data gatherer is intended to be used for reading data for evaluation
from the local file system. It can also be used for 'stubbing' remote data
sources when testing other data gatherers.

## Configuration

Stubbing another datagatherer for testing:

```yaml
data-gatherers:
- kind: "gke"
  name: "gke"
  config:
    # fetch from local path instead of GKE
    data-path: ./examples/data/example.json
```

Loading other data as 'local':

```yaml
data-gatherers:
- kind: "local"
  name: "local"
  config:
    data-path: ./examples/data/example.json
```

## Data

Data is gathered from the local file system - whatever is read from the file is
used.

## Permissions

Permissions to read the local path.
