# venafi-kubernetes-agent

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.

![Version: 0.1.48](https://img.shields.io/badge/Version-0.1.48-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.48](https://img.shields.io/badge/AppVersion-v0.1.48-informational?style=flat-square)

## Additional Information

The Venafi Kubernetes Agent connects your Kubernetes or OpenShift cluster to the Venafi Control Plane.
You will require a Venafi Control Plane account to connect your cluster.
If you do not have one, you can sign up for a free trial now at:
- https://venafi.com/try-venafi/tls-protect/

Note that there are EU and US Venafi Control Plane options.
Upon signing up you will be redirected to one of either of the following login URLs:
- https://ui.venafi.cloud/ (US)
- https://ui.venafi.eu/ (EU)

> 📖 Learn more about [Venafi Kubernetes Agent network requirements](https://docs.venafi.cloud/vaas/k8s-components/c-vcp-network-requirements/),
> in the two regions.

## Installation

The Helm chart is available from the following Venafi OCI registries:

- `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent` (public)
- `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent` (private, US)
- `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent` (private, EU)

> ℹ️ In the following steps it is assumed that you are using the **public** registry.
>
> 📖 Learn [how to access the private Venafi OCI registries](https://docs.venafi.cloud/vaas/k8s-components/th-guide-confg-access-to-tlspk-enterprise-components/).

Familiarise yourself with the Helm chart:

```sh
helm show readme oci://registry.venafi.cloud/charts/venafi-kubernetes-agent
helm show values oci://registry.venafi.cloud/charts/venafi-kubernetes-agent
helm template oci://registry.venafi.cloud/charts/venafi-kubernetes-agent
```

### 1) Create a Venafi service account

Create a new service account in the Venafi TLS Protect Cloud web UI.
The service account is used by the Venafi Kubernetes Agent to authenticate to the Venafi Control Plane.
Every Venafi Kubernetes Agent should use a unique service account.
You must create the service account **before** installing the Helm chart.

First create an RSA key pair:

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
- Find the row matching the name you entered and copy the "Client ID" value,
  by clicking "Copy Client ID" in the row actions menu.
  You will need this when you install the Helm chart.

### 2) Deploy the chart

Create a Namespace and then create a Secret containing the private key of the service account:

```shell
export VENAFI_NAMESPACE="venafi"
kubectl create namespace ${VENAFI_NAMESPACE}
kubectl create secret generic agent-credentials \
  --namespace ${VENAFI_NAMESPACE} \
  --from-file=privatekey.pem=${VENAFI_SERVICE_ACCOUNT}.pem
```

Install the chart:

```shell
export VENAFI_CLIENT_ID="<your-client-id>"
helm upgrade venafi-kubernetes-agent oci://registry.venafi.cloud/charts/venafi-kubernetes-agent \
  --install \
  --namespace ${VENAFI_NAMESPACE} \
  --set config.clientId="${VENAFI_CLIENT_ID}"
```

> ℹ️ To use the [EU Venafi Control Plane](https://docs.venafi.cloud/vaas/k8s-components/c-vcp-network-requirements/),
> add: `--set config.server=https://api.venafi.eu/`.

### 3) Connect the cluster in Venafi Control Plane

- Click **Installations > Kubernetes Clusters**.
- Click **Connect**.
- On step 1, click **Continue**.
- On step 2, select **Advanced Connection**.
- On step 3, click **Continue** to skip.
- On step 4, fill in the details as follows:
  - Name: use the name of the service account that you created earlier. E.g. "example-cluster".
  - Service Account: select the service account that you created earlier.
  - Check "The connection command has completed." box and click **continue**.
- On step 5, either wait for validation or click **Finish** to go back to the cluster list.

### 4) Verify the deployment

Check the agent logs:

```shell
kubectl logs -n ${VENAFI_NAMESPACE} -l app.kubernetes.io/instance=venafi-kubernetes-agent --tail -1 | grep -A 5 "Running Agent"
```

You should see:

```console
2023/10/24 12:10:03 Running Agent...
2023/10/24 12:10:03 Posting data to: https://api.venafi.cloud/
2023/10/24 12:10:04 Data sent successfully.
```

Check the cluster status by visiting the Clusters page in the Venafi Control Plane:
- Click  **Installations > Kubernetes Clusters**

You should see:
- Status: Active
- Last Check In: ...seconds ago

Check the Event Log page:
- Click **Settings > Event Log**

You should see the following events for your service account:
- Service Account Access Token Granted
- Login Succeeded

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Embed YAML for Node affinity settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/. |
| authentication | object | `{"secretKey":"privatekey.pem","secretName":"agent-credentials"}` | Authentication details for the Venafi Kubernetes Agent |
| authentication.secretKey | string | `"privatekey.pem"` | Key name in the referenced secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing the private key |
| command | list | `[]` | Specify the command to run overriding default binary. |
| config | object | `{"clientId":"","clusterDescription":"","clusterName":"","configmap":{"key":null,"name":null},"period":"0h1m0s","server":"https://api.venafi.cloud/"}` | Configuration section for the Venafi Kubernetes Agent itself |
| config.clientId | string | `""` | The client-id returned from the Venafi Control Plane |
| config.clusterDescription | string | `""` | Description for the cluster resource if it needs to be created in Venafi Control Plane |
| config.clusterName | string | `""` | Name for the cluster resource if it needs to be created in Venafi Control Plane |
| config.configmap | object | `{"key":null,"name":null}` | Specify ConfigMap details to load config from an existing resource. This should be blank by default unless you have you own config. |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://api.venafi.cloud/"` | Overrides the server if using a proxy in your environment For the EU variant use: https://api.venafi.eu/ |
| extraArgs | list | `[]` | Specify additional arguments to pass to the agent binary. For example `["--strict", "--oneshot"]` |
| fullnameOverride | string | `""` | Helm default setting, use this to shorten the full install name. |
| image.pullPolicy | string | `"IfNotPresent"` | Defaults to only pull if not already present |
| image.repository | string | `"registry.venafi.cloud/venafi-agent/venafi-agent"` | Default to Open Source image repository |
| image.tag | string | `"v0.1.48"` | Overrides the image tag whose default is the chart appVersion |
| imagePullSecrets | list | `[]` | Specify image pull credentials if using a private registry example: - name: my-pull-secret |
| nameOverride | string | `""` | Helm default setting to override release name, usually leave blank. |
| nodeSelector | object | `{}` | Embed YAML for nodeSelector settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/ |
| podAnnotations | object | `{}` | Additional YAML annotations to add the the pod. |
| podDisruptionBudget | object | `{"enabled":false}` | Configure a PodDisruptionBudget for the agent's Deployment. If running with multiple replicas, consider setting podDisruptionBudget.enabled to true. |
| podDisruptionBudget.enabled | bool | `false` | Enable or disable the PodDisruptionBudget resource, which helps prevent downtime during voluntary disruptions such as during a Node upgrade. |
| podSecurityContext | object | `{}` | Optional Pod (all containers) `SecurityContext` options, see https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod. |
| replicaCount | int | `1` | default replicas, do not scale up |
| resources | object | `{"limits":{"memory":"500Mi"},"requests":{"cpu":"200m","memory":"200Mi"}}` | Set resource requests and limits for the pod.  Read [Venafi Kubernetes components deployment best practices](https://docs.venafi.cloud/vaas/k8s-components/c-k8s-components-best-practice/#scaling) to learn how to choose suitable CPU and memory resource requests and limits. |
| securityContext | object | `{"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"runAsUser":1000}` | Add Container specific SecurityContext settings to the container. Takes precedence over `podSecurityContext` when set. See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container |
| serviceAccount.annotations | object | `{}` | Annotations YAML to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If blank and `serviceAccount.create` is true, a name is generated using the fullname template of the release. |
| tolerations | list | `[]` | Embed YAML for toleration settings, see https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |

