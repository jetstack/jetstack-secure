# venafi-kubernetes-agent

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.40](https://img.shields.io/badge/AppVersion-v0.1.40-informational?style=flat-square)

## Additional Information

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.
You will require a Venafi Control Plane account to connect your cluster.
If you do not have you, you can sign up for a free trial now at:
- https://venafi.com/try-venafi/tls-protect/

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
| authentication | object | `{"secretKey":"privatekey.pem","secretName":"agent-credentials"}` | Authentication details for the Venafi Kuberente Agent |
| authentication.secretKey | string | `"privatekey.pem"` | Key name in the references secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing the privatekey |
| command | list | `[]` | Specify the command to run overriding default |
| config | object | `{"clientId":"","configmap":{"key":null,"name":null},"period":"0h1m0s","server":"https://api.venafi.cloud/"}` | Configuration section for the Venafi Kubernetes Agent itself |
| config.configmap | object | `{"key":null,"name":null}` | Sepcify ConfigMap details to load config from an existing resource This should be blankby default unless you have you own config |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://api.venafi.cloud/"` | Overrides the server if using a proxy in your environment |
| extraArgs | list | `[]` | Specify additional argument to pass to the agent |
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
Autogenerated from chart metadata using [helm-docs v1.11.0](https://github.com/norwoodj/helm-docs/releases/v1.11.0)
