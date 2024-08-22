# venafi-kubernetes-agent

The Venafi Kubernetes Agent connects your Kubernetes or Openshift cluster to the Venafi Control Plane.

![Version: 0.1.49](https://img.shields.io/badge/Version-0.1.49-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.49](https://img.shields.io/badge/AppVersion-v0.1.49-informational?style=flat-square)

The Venafi Kubernetes Agent connects your Kubernetes or OpenShift cluster to the Venafi Control Plane.
You will require a Venafi Control Plane account to connect your cluster.
If you do not have one, you can sign up for a free trial now at:
- https://venafi.com/try-venafi/tls-protect/

> 📖 Read the [Venafi Kubernetes Agent documentation](https://docs.venafi.cloud/vaas/k8s-components/c-tlspk-agent-overview/),
> to learn how install and configure this Helm chart.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Embed YAML for Node affinity settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/. |
| authentication | object | `{"secretKey":"privatekey.pem","secretName":"agent-credentials","venafiConnection":{"enabled":false,"name":"venafi-components","namespace":"venafi"}}` | Authentication details for the Venafi Kubernetes Agent |
| authentication.secretKey | string | `"privatekey.pem"` | Key name in the referenced secret |
| authentication.secretName | string | `"agent-credentials"` | Name of the secret containing the private key |
| authentication.venafiConnection | object | `{"enabled":false,"name":"venafi-components","namespace":"venafi"}` | Configure VenafiConnection authentication |
| authentication.venafiConnection.enabled | bool | `false` | When set to true, the Venafi Kubernetes Agent will authenticate to Venafi using the configuration in a VenafiConnection resource. Use `venafiConnection.enabled=true` for [secretless authentication](https://docs.venafi.cloud/vaas/k8s-components/t-install-tlspk-agent/). When set to true, the `authentication.secret` values will be ignored and the Secret with `authentication.secretName` will _not_ be mounted into the Venafi Kubernetes Agent Pod. |
| authentication.venafiConnection.name | string | `"venafi-components"` | The name of a VenafiConnection resource which contains the configuration for authenticating to Venafi. |
| authentication.venafiConnection.namespace | string | `"venafi"` | The namespace of a VenafiConnection resource which contains the configuration for authenticating to Venafi. |
| command | list | `[]` | Specify the command to run overriding default binary. |
| config | object | `{"clientId":"","clusterDescription":"","clusterName":"","configmap":{"key":null,"name":null},"ignoredSecretTypes":["kubernetes.io/service-account-token","kubernetes.io/dockercfg","kubernetes.io/dockerconfigjson","kubernetes.io/basic-auth","kubernetes.io/ssh-auth","bootstrap.kubernetes.io/token","helm.sh/release.v1"],"period":"0h1m0s","server":"https://api.venafi.cloud/"}` | Configuration section for the Venafi Kubernetes Agent itself |
| config.clientId | string | `""` | The client-id returned from the Venafi Control Plane |
| config.clusterDescription | string | `""` | Description for the cluster resource if it needs to be created in Venafi Control Plane |
| config.clusterName | string | `""` | Name for the cluster resource if it needs to be created in Venafi Control Plane |
| config.configmap | object | `{"key":null,"name":null}` | Specify ConfigMap details to load config from an existing resource. This should be blank by default unless you have you own config. |
| config.ignoredSecretTypes | list | `["kubernetes.io/service-account-token","kubernetes.io/dockercfg","kubernetes.io/dockerconfigjson","kubernetes.io/basic-auth","kubernetes.io/ssh-auth","bootstrap.kubernetes.io/token","helm.sh/release.v1"]` | Reduce the memory usage of the agent and reduce the load on the Kubernetes API server by omitting various common Secret types when listing Secrets. These Secret types will be added to a "type!=<type>" field selector in the agent config. * https://docs.venafi.cloud/vaas/k8s-components/t-cfg-tlspk-agent/#configuration * https://kubernetes.io/docs/concepts/configuration/secret/#secret-types * https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/#list-of-supported-fields |
| config.period | string | `"0h1m0s"` | Send data back to the platform every minute unless changed |
| config.server | string | `"https://api.venafi.cloud/"` | Overrides the server if using a proxy in your environment For the EU variant use: https://api.venafi.eu/ |
| crds.forceRemoveValidationAnnotations | bool | `false` | The 'x-kubernetes-validations' annotation is not supported in Kubernetes 1.22 and below. This annotation is used by CEL, which is a feature introduced in Kubernetes 1.25 that improves how validation is performed. This option allows to force the 'x-kubernetes-validations' annotation to be excluded, even on Kubernetes 1.25+ clusters. |
| crds.venafiConnection | object | `{"include":false}` | Optionally include the VenafiConnection CRDs |
| crds.venafiConnection.include | bool | `false` | When set to false, the rendered output does not contain the VenafiConnection CRDs and RBAC. This is useful for when the Venafi Connection resources are already installed separately. |
| extraArgs | list | `[]` | Specify additional arguments to pass to the agent binary. For example `["--strict", "--oneshot"]` |
| fullnameOverride | string | `""` | Helm default setting, use this to shorten the full install name. |
| image.pullPolicy | string | `"IfNotPresent"` | Defaults to only pull if not already present |
| image.repository | string | `"registry.venafi.cloud/venafi-agent/venafi-agent"` | Default to Open Source image repository |
| imagePullSecrets | list | `[]` | Specify image pull credentials if using a private registry example: - name: my-pull-secret |
| metrics.enabled | bool | `true` | Enable the metrics server. If false, the metrics server will be disabled and the other metrics fields below will be ignored. |
| metrics.podmonitor.annotations | object | `{}` | Additional annotations to add to the PodMonitor. |
| metrics.podmonitor.enabled | bool | `false` | Create a PodMonitor to add the metrics to Prometheus, if you are using Prometheus Operator. See https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.PodMonitor |
| metrics.podmonitor.endpointAdditionalProperties | object | `{}` | EndpointAdditionalProperties allows setting additional properties on the endpoint such as relabelings, metricRelabelings etc.  For example:  endpointAdditionalProperties:   relabelings:   - action: replace     sourceLabels:     - __meta_kubernetes_pod_node_name     targetLabel: instance  |
| metrics.podmonitor.honorLabels | bool | `false` | Keep labels from scraped data, overriding server-side labels. |
| metrics.podmonitor.interval | string | `"60s"` | The interval to scrape metrics. |
| metrics.podmonitor.labels | object | `{}` | Additional labels to add to the PodMonitor. |
| metrics.podmonitor.prometheusInstance | string | `"default"` | Specifies the `prometheus` label on the created PodMonitor. This is used when different Prometheus instances have label selectors matching different PodMonitors. |
| metrics.podmonitor.scrapeTimeout | string | `"30s"` | The timeout before a metrics scrape fails. |
| nameOverride | string | `""` | Helm default setting to override release name, usually leave blank. |
| nodeSelector | object | `{}` | Embed YAML for nodeSelector settings, see https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/ |
| podAnnotations | object | `{}` | Additional YAML annotations to add the the pod. |
| podDisruptionBudget | object | `{"enabled":false}` | Configure a PodDisruptionBudget for the agent's Deployment. If running with multiple replicas, consider setting podDisruptionBudget.enabled to true. |
| podDisruptionBudget.enabled | bool | `false` | Enable or disable the PodDisruptionBudget resource, which helps prevent downtime during voluntary disruptions such as during a Node upgrade. |
| podSecurityContext | object | `{}` | Optional Pod (all containers) `SecurityContext` options, see https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod. |
| replicaCount | int | `1` | default replicas, do not scale up |
| resources | object | `{"limits":{"memory":"500Mi"},"requests":{"cpu":"200m","memory":"200Mi"}}` | Set resource requests and limits for the pod.  Read [Venafi Kubernetes components deployment best practices](https://docs.venafi.cloud/vaas/k8s-components/c-k8s-components-best-practice/#scaling) to learn how to choose suitable CPU and memory resource requests and limits. |
| securityContext | object | `{"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true}` | Add Container specific SecurityContext settings to the container. Takes precedence over `podSecurityContext` when set. See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container |
| serviceAccount.annotations | object | `{}` | Annotations YAML to add to the service account |
| serviceAccount.create | bool | `true` | Specifies whether a service account should be created |
| serviceAccount.name | string | `""` | The name of the service account to use. If blank and `serviceAccount.create` is true, a name is generated using the fullname template of the release. |
| tolerations | list | `[]` | Embed YAML for toleration settings, see https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| volumeMounts | list | `[]` | Additional volume mounts to add to the Venafi Kubernetes Agent container. This is useful for mounting a custom CA bundle. Any PEM certificate mounted under /etc/ssl/certs will be loaded by the Venafi Kubernetes Agent. For example:      volumeMounts:       - name: cabundle         mountPath: /etc/ssl/certs/cabundle         subPath: cabundle         readOnly: true |
| volumes | list | `[]` | Additional volumes to add to the Venafi Kubernetes Agent container. This is useful for mounting a custom CA bundle. For example:      volumes:       - name: cabundle         configMap:           name: cabundle           optional: false           defaultMode: 0644  In order to create the ConfigMap, you can use the following command:      kubectl create configmap cabundle \       --from-file=cabundle=./your/custom/ca/bundle.pem |

