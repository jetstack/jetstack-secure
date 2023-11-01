# venafi-kubernetes-agent

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.43](https://img.shields.io/badge/AppVersion-v0.1.43-informational?style=flat-square)

## Additional Information

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.
You will require a Venafi Control Plane account to connect your cluster.
If you do not have you, you can sign up for a free trial now at:
- https://venafi.com/try-venafi/tls-protect/

## Installation:

Using chart installation, there are two credentials required.

1) A registry credential to allow helm to pull the chart from our private OCI registry.
2) A service acccount key pair used by the agent to authenticate to the Venafi Control Plane.

### 1) Setup registry credentials

The helm chart is an OCI chart artifact hosted on both EU and US registries:

- `oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent`
- `oci://us.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent`

More detailed instructions on how to access our registry are available in [this guide](https://platform.jetstack.io/documentation/installation/enterprise-registry).

For chart installation, run the following to set a registry configuration
file, so `helm` can authenticate to our private OCI registry:

```shell
export VENAFI_DOCKER_CONFIG_PATH="$(pwd)"
export VENAFI_DOCKER_CONFIG_FILE="${TLSPK_DOCKER_CONFIG_PATH}/config.json"
jsctl registry auth output --format=dockerconfig > "${VENAFI_DOCKER_CONFIG_FILE}"
```

To validate you registry credentials are working with `helm`, we can use it to
show us the full list of values available to configure the chart:

```shell
export VENAFI_REGISTRY="eu.gcr.io/jetstack-secure-enterprise"
helm show values oci://${VENAFI_REGISTRY}/charts/venafi-kubernetes-agent \
  --registry-config "${VENAFI_DOCKER_CONFIG_FILE}"
```

**Note**: Feel free to alter the registry to the US equivalent if that is closerto you.
For example: `export VENAFI_REGISTRY="us.gcr.io/jetstack-secure-enterprise"`

### 2) Creating Venafi Service Account:

First we need to create an OpenSSL key pair and save the private key securely.
The private key is used by the agent and you should have a unique key for each agent you connect to the Venafi Control Plane.
The public key will be added to the Venafi Control Plane as the service account credential and assigned to the appropriate team for ownership.

```shell
export VENAFI_NAMESPACE="venafi" VENAFI_SERVICE_ACCOUNT="example-cluster"
openssl genrsa -out ${VENAFI_SERVICE_ACCOUNT}.pem
openssl rsa -in ${VENAFI_SERVICE_ACCOUNT}.pem -pubout --out ${VENAFI_SERVICE_ACCOUNT}.pub
```

Now that you have both the private and public key we now need to use the Venafi Control Plane to create a service account.

- Navigate to the [service accounts page](https://ui.venafi.cloud/service-accounts/) and select "New"
- Add a unique name matching the variable we used, eg: "example-cluster"
- Assign a team that owns this credential
- The scope should be "Kuberentes Discovery" only.
- Set the validity period of your pubic key up to a maximum of 365 days.
- Now paste in the **public key** from the pair you generated.

Once created, you will be returned to the service accounts list.
Find your newest entry matching the name you entered, and copy the "Client ID" column.

### 3) Deploying the chart:

Now we have the service account, let us prepare a namespace with the relevant private key needed at runtime.

```shell
export VENAFI_CLIENT_ID="<PASTE YOURS HERE>"
kubectl create namespace ${VENAFI_NAMESPACE}
kubectl create secret generic agent-credentials -n ${VENAFI_NAMESPACE} \
  --from-file=privatekey.pem=${VENAFI_SERVICE_ACCOUNT}.pem
```

Install the chart by setting the `config.clientId` field:

```shell
helm upgrade --install agent deploy/charts/venafi-kubernetes-agent \
  --namespace ${VENAFI_NAMESPACE} \
  --set config.clientId="${VENAFI_CLIENT_ID}"
```

### 4) Add Cluster in Venafi Control Plane

- Go to "Installations" -> "Kuberentes Clusters" [here](https://ui.venafi.cloud/clusters-inventory) and click "Connect"
- On step 1 select "Continue".
- On step 2 select "Advanced Connection".
- On step 3 select "Continue" to skip.
- On step 4, fill in the details as needed:
  - "Name" should match your service account name from before, eg "example-cluster".
  - Under "Service Account" click that drop down and select the previously created service account.
  - Then check the "The connection command has completed." box and select "continue".
- On step 5, either wait for validation or select "Finish" to go back to the cluster list.

### 5) Deployment Verification

Check the agent logs to ensure you see a similar entry to the following:

```console
2023/10/24 12:10:03 Running Agent...
2023/10/24 12:10:03 Posting data to: https://api.venafi.cloud/
2023/10/24 12:10:04 Data sent successfully.
```

You can do this with the following command:

```shell
kubectl logs -n ${VENAFI_NAMESPACE} $(kubectl get pod -n ${VENAFI_NAMESPACE} -l app.kubernetes.io/instance=agent -o jsonpath='{.items[0].metadata.name}')
```

You can also check inb the Venafi Control Plane to see when the "Last Check In" was for your cluster.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Embed YAML for Node affinity settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/. |
| authentication | object | `{"secretKey":"privatekey.pem","secretName":"agent-credentials"}` | Authentication details for the Venafi Kubernetes Agent |
| authentication.secretKey | string | `"privatekey.pem"` | Key name in the referenced secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing the privatekey |
| command | list | `[]` | Specify the command to run overriding default binary. |
| config | object | `{"clientId":"","configmap":{"key":null,"name":null},"period":"0h1m0s","server":"https://api.venafi.cloud/"}` | Configuration section for the Venafi Kubernetes Agent itself |
| config.clientId | string | `""` | The client-id returned from the Venafi Control Plane |
| config.configmap | object | `{"key":null,"name":null}` | Specify ConfigMap details to load config from an existing resource. This should be blank by default unless you have you own config.  |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://api.venafi.cloud/"` | Overrides the server if using a proxy in your environmen For the EU varint use: https://api.venafi.eu/ |
| extraArgs | list | `[]` | Specify additional arguments to pass to the agent binary. For example `["--strict", "--oneshot"]` |
| fullnameOverride | string | `""` | Helm default setting, use this to shorten the full install name. |
| image.pullPolicy | string | `"IfNotPresent"` | Defaults to only pull if not already present |
| image.repository | string | `"quay.io/jetstack/preflight"` | Default to Open Source image repository |
| image.tag | string | `"v0.1.43"` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Specify image pull credentials if using a prviate registry example: - name: my-pull-secret |
| nameOverride | string | `""` | Helm default setting to override release name, usually leave blank. |
| nodeSelector | object | `{}` | Embed YAML for nodeSelector settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/ |
| podAnnotations | object | `{}` | Additional YAML annotations to add the the pod. |
| podSecurityContext | object | `{}` | Optional Pod (all containers) `SecurityContext` options, see https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod. |
| replicaCount | int | `1` | default replicas, do not scale up |
| resources | object | `{"limits":{"cpu":"500m","memory":"500Mi"},"requests":{"cpu":"200m","memory":"200Mi"}}` | Set custom resourcing settings for the pod. You may not want this if you intend to use a Vertical Pod Autoscaler. |
| securityContext | object | `{"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"runAsUser":1000}` | Add Container specific SecurityContext settings to the container. Takes precedence over `podSecurityContext` when set. See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container |
| serviceAccount.annotations | object | `{}` | Annotations YAML to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If blank and `serviceAccount.create` is true, a name is generated using the fullname template of the release. |
| tolerations | list | `[]` | Embed YAML for toleration settings, see https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.3](https://github.com/norwoodj/helm-docs/releases/v1.11.3)
