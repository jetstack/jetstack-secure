# jetstack-agent

TLS Protect for Kubernetes Agent

![Version: 0.3.0](https://img.shields.io/badge/Version-0.3.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.40](https://img.shields.io/badge/AppVersion-v0.1.40-informational?style=flat-square)

## Additional Information

The Jetstack Secure agent helm chart installs the Kubernetes agent that connects to the TLS Protect For Kubernetes (TLSPK) platform.
It will require a valid TLS Protect for Kubernetes organisation with a license to add the new cluster.
You can sign up for a free account with up to two clusters [here](https://platform.jetstack.io/).
You should also choose a unique name for your cluster that it will appear under in the TLSPK platform.

## Installation:

Using chart installation, there are two credentials required.

- A credential to allow helm to pull the chart from our private OCI registry.
- An agent credential used by the agent to authenticate to TLSPK.

### 1) Obtain OCI registry credentials

The helm chart is an OCI chart artifact hosted on both EU and US registries:

- `oci://eu.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`
- `oci://us.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`

More detailed instructions on how to access our registry are available in [this guide](https://platform.jetstack.io/documentation/installation/enterprise-registry).

For chart installation, run the following to set a registry configuration
file, so `helm` can authenticate to our private OCI registry:

```shell
export TLSPK_DOCKER_CONFIG_PATH="$(pwd)"
export TLSPK_DOCKER_CONFIG_FILE="${TLSPK_DOCKER_CONFIG_PATH}/config.json"
jsctl registry auth output --format=dockerconfig > "${TLSPK_DOCKER_CONFIG_FILE}"
```

To validate you registry credentials are working with `helm`, we can use it to
show us the full list of values available to configure the chart:

```shell
export TLSPK_REGISTRY="eu.gcr.io/jetstack-secure-enterprise"
helm show values oci://${TLSPK_REGISTRY}/charts/jetstack-agent --registry-config "${TLSPK_DOCKER_CONFIG_FILE}"
```

**Note**: Feel free to alter the registry to the US equivalent if that is closer
to you, for example: `export TLSPK_REGISTRY="us.gcr.io/jetstack-secure-enterprise"`

### 2) Obtaining TLSPK agent credentials:

Set the following environments variables for ease of installation:

```shell
export TLSPK_ORG="<ORG_NAME>"
export TLSPK_CLUSTER_NAME="<CLUSTER_NAME>"
```

Obtain your service account credential, this can be done through the UI or [jsctl](https://github.com/jetstack/jsctl/releases)

For example with `jsctl`:

```shell
jsctl auth login
jsctl set organization ${TLSPK_ORG}
jsctl auth clusters create-service-account ${TLSPK_CLUSTER_NAME} | tee credentials.json
```

Store this carefully as we will need it to create a Kubernetes secret in the
installation cluster.

### 3) Deploying the chart:

Once credentials are obtained, there are two ways to install the chart:

#### Option 1 (Recommended): Create secret manually

Use the credential obtained in the previous step to create the secret in cluster:

```shell
kubectl create secret generic agent-credentials --namespace jetstack-secure --from-file=credentials.json
```

Install the chart with the basic configuration:

```shell
helm upgrade --install --create-namespace -n jetstack-secure jetstack-agent \
  oci://${TLSPK_REGISTRY}/charts/jetstack-agent \
  --registry-config "${TLSPK_DOCKER_CONFIG_FILE}" \
  --set config.organisation="${TLSPK_ORG}" \
  --set config.cluster="${TLSPK_CLUSTER_NAME}"
```

#### Option 2 (Not Recommended): Create secret with helm chart install

Set this environment variable to contain the encoded agent credential:

```shell
export HELM_SECRET="$(cat credentials.json | base64 -w0)"
```

Installing the chart with additional configuration options for the agents
credential, read from the environment variable just set:

```shell
helm upgrade --install --create-namespace -n jetstack-secure jetstack-agent \
  oci://${TLSPK_REGISTRY}/charts/jetstack-agent \
  --registry-config "${TLSPK_DOCKER_CONFIG_FILE}" \
  --set config.organisation="${TLSPK_ORG}" \
  --set config.cluster="${TLSPK_CLUSTER_NAME}" \
  --set authentication.createSecret=true \
  --set authentication.secretValue="${HELM_SECRET}"
```

### 4) Deployment Verification

Check the agent logs to ensure you see a similar entry to the following:

```console
2023/04/19 14:11:41 Running Agent...
2023/04/19 14:11:41 Posting data to: https://platform.jetstack.io
2023/04/19 14:11:42 Data sent successfully.
```

You can do this with the following command:

```shell
kubectl logs -n jetstack-secure $(kubectl get pod -n jetstack-secure -l app.kubernetes.io/instance=agent -o jsonpath='{.items[0].metadata.name}')
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| authentication.createSecret | bool | `false` | Reccomend that you do not use this and instead creat the credential secret outside of helm |
| authentication.secretKey | string | `"credentials.json"` | Key name in secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing agent credentials.json |
| authentication.secretValue | string | `""` | Base64 encoded value from Jetstack Secure Dashboard - only required when createSecret is true |
| authentication.type | string | `"file"` | Type can be "file"/"token" determining how the agent should authenticate the to the backend |
| command | list | `[]` |  |
| config | object | `{"cluster":"","dataGatherers":{"custom":[],"default":true},"organisation":"","override":{"config":null,"configmap":{"key":null,"name":null},"enabled":false},"period":"0h1m0s","server":"https://platform.jetstack.io"}` | Configuration section for the Jetstack Agent itself |
| config.cluster | string | `""` | REQUIRED - Your Jetstack Secure Cluster Name |
| config.dataGatherers | object | `{"custom":[],"default":true}` | Configure data that is gathered from your cluster, for full details see https://platform.jetstack.io/documentation/configuration/jetstack-agent/configuration |
| config.dataGatherers.custom | list | `[]` | A list of data gatherers to limit agent scope |
| config.dataGatherers.default | bool | `true` | Use the standard full set of data gatherers |
| config.organisation | string | `""` | REQUIRED - Your Jetstack Secure Organisation Name |
| config.override | object | `{"config":null,"configmap":{"key":null,"name":null},"enabled":false}` | Provide an Override to allow completely custom agent configuration |
| config.override.config | string | `nil` | Embed the agent configuration here in the chart values |
| config.override.configmap | object | `{"key":null,"name":null}` | Sepcify ConfigMap details to load config from existing ConfigMap |
| config.override.enabled | bool | `false` | Override disabled by default |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://platform.jetstack.io"` | Overrides the server if using a proxy between agent and Jetstack Secure |
| extraArgs | list | `[]` |  |
| fullnameOverride | string | `""` | Helm default setting, use this to shorten install name |
| image.pullPolicy | string | `"IfNotPresent"` | Defaults to only pull if not already present |
| image.repository | string | `"quay.io/jetstack/preflight"` | Default to Open Source image repository |
| image.tag | string | `"v0.1.40"` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Specify image pull credentials if using a prviate registry |
| nameOverride | string | `""` | Helm default setting to override release name, leave blank |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| replicaCount | int | `1` | default replicas, do not scale up |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"500Mi"` |  |
| resources.requests.cpu | string | `"200m"` |  |
| resources.requests.memory | string | `"200Mi"` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| securityContext.readOnlyRootFilesystem | bool | `true` |  |
| securityContext.runAsNonRoot | bool | `true` |  |
| securityContext.runAsUser | int | `1000` |  |
| serviceAccount.annotations | object | `{}` | Annotations to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created @default true |
| serviceAccount.name | string | `""` |  |
| tolerations | list | `[]` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.3](https://github.com/norwoodj/helm-docs/releases/v1.11.3)
