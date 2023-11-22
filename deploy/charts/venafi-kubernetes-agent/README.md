# venafi-kubernetes-agent

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.

![Version: 0.1.43-alpha.0](https://img.shields.io/badge/Version-0.1.43--alpha.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.43](https://img.shields.io/badge/AppVersion-v0.1.43-informational?style=flat-square)

## Additional Information

The Venafi Kubernetes Agent connects your Kubernetes or OpenShift cluster to the Venafi Control Plane.
You will require a Venafi Control Plane account to connect your cluster.
If you do not have one, you can sign up for a free trial now at:
- https://venafi.com/try-venafi/tls-protect/

Note that there are EU and US Venafi Control Plane options.
Upon signing up you will be redirected to one of either of the following login URLs:
- https://ui.venafi.cloud/ (US)
- https://ui.venafi.eu/ (EU)

## Installation

The Helm chart is available from the following Venafi OCI registries:

- `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent` (public)
- `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent` (private, US)
- `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent` (private, EU)

> Learn [how to access the private Venafi OCI registries](https://docs.venafi.cloud/vaas/k8s-components/th-guide-confg-access-to-tlspk-enterprise-components/).

Before installing the chart you will need a service account key pair,
which is used by the agent to authenticate to the Venafi Control Plane.

First create an RSA key pair and save the private key securely.
The private key is used by the agent and you should have a unique key for each agent you connect to the Venafi Control Plane.
The public key will be added to the Venafi Control Plane as the service account credential and assigned to the appropriate team for ownership.

```shell
export VENAFI_SERVICE_ACCOUNT="example-cluster"
openssl genrsa -out ${VENAFI_SERVICE_ACCOUNT}.pem
openssl rsa -in ${VENAFI_SERVICE_ACCOUNT}.pem -pubout --out ${VENAFI_SERVICE_ACCOUNT}.pub
```

Next create a service account in the Venafi Control Plane:

- Click **Settings > Service Accounts**.
- Click **New**.
- Type a name for your new service account.
  Must match the ${VENAFI_SERVICE_ACCOUNT} variable that you used above.
- Select an **Owning Team**, which is the team who owns the machine you want to create the service account for.
- The scope should be "Kubernetes Discovery" only.
- Set the validity period of your pubic key up to a maximum of 365 days.
- Paste in the **public key** from the pair you generated.
- Click **Save** to finish and return to the Service Account list view.
- Find the newest entry matching the name you entered and copy the "Client ID" value.

### 3) Deploying the chart:

Now prepare a Namespace and a Secret containing the private key of the service account:

```shell
export VENAFI_NAMESPACE="venafi"
kubectl create namespace ${VENAFI_NAMESPACE}
kubectl create secret generic agent-credentials \
  --namespace ${VENAFI_NAMESPACE} \
  --from-file=privatekey.pem=${VENAFI_SERVICE_ACCOUNT}.pem
```

Install the chart by setting the `config.clientId` field:

```shell
export VENAFI_CLIENT_ID="fd93a1e5-8968-11ee-916c-3e98640ed54f"
helm upgrade venafi-kubernetes-agent oci://registry.venafi.cloud/charts/venafi-kubernetes-agent \
  --install \
  --namespace ${VENAFI_NAMESPACE} \
  --set config.clientId="${VENAFI_CLIENT_ID}"
```

> To change the backend to the EU Venafi Control Plane, use the following Helm value:
> `--set config.server="${VENAFI_SERVER_URL}"`

### 4) Add Cluster in Venafi Control Plane

- Go to "Installations" -> "Kubernetes Clusters" [here](https://ui.venafi.cloud/clusters-inventory) and click "Connect". **Note** you may need to click [here](https://ui.venafi.eu/clusters-inventory) for the EU backend.
- On step 1 select "Continue".
- On step 2 select "Advanced Connection".
- On step 3 select "Continue" to skip.
- On step 4, fill in the details as needed:
  - "Name" should match your service account name from before, e.g. "example-cluster".
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
kubectl logs -n ${VENAFI_NAMESPACE} $(kubectl get pod -n ${VENAFI_NAMESPACE} -l app.kubernetes.io/instance=venafi-kubernetes-agent -o jsonpath='{.items[0].metadata.name}')
```

You can also check in the Venafi Control Plane to see when the "Last Check In" was for your cluster.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Embed YAML for Node affinity settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/. |
| authentication | object | `{"secretKey":"privatekey.pem","secretName":"agent-credentials"}` | Authentication details for the Venafi Kubernetes Agent |
| authentication.secretKey | string | `"privatekey.pem"` | Key name in the referenced secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing the private key |
| command | list | `[]` | Specify the command to run overriding default binary. |
| config | object | `{"clientId":"","configmap":{"key":null,"name":null},"period":"0h1m0s","server":"https://api.venafi.cloud/"}` | Configuration section for the Venafi Kubernetes Agent itself |
| config.clientId | string | `""` | The client-id returned from the Venafi Control Plane |
| config.configmap | object | `{"key":null,"name":null}` | Specify ConfigMap details to load config from an existing resource. This should be blank by default unless you have you own config.  |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://api.venafi.cloud/"` | Overrides the server if using a proxy in your environment For the EU variant use: https://api.venafi.eu/ |
| extraArgs | list | `[]` | Specify additional arguments to pass to the agent binary. For example `["--strict", "--oneshot"]` |
| fullnameOverride | string | `""` | Helm default setting, use this to shorten the full install name. |
| image.pullPolicy | string | `"IfNotPresent"` | Defaults to only pull if not already present |
| image.repository | string | `"quay.io/jetstack/preflight"` | Default to Open Source image repository |
| image.tag | string | `"v0.1.43"` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Specify image pull credentials if using a private registry example: - name: my-pull-secret |
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

