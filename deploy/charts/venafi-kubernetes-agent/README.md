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

<!-- AUTO-GENERATED -->

#### **metrics.enabled** ~ `bool`
> Default value:
> ```yaml
> true
> ```

Enable the metrics server.  
If false, the metrics server will be disabled and the other metrics fields below will be ignored.
#### **metrics.podmonitor.enabled** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Create a PodMonitor to add the metrics to Prometheus, if you are using Prometheus Operator. See https://prometheus-operator.dev/docs/operator/api/#monitoring.coreos.com/v1.PodMonitor
#### **metrics.podmonitor.namespace** ~ `string`

The namespace that the pod monitor should live in. Defaults to the venafi-kubernetes-agent namespace.

#### **metrics.podmonitor.prometheusInstance** ~ `string`
> Default value:
> ```yaml
> default
> ```

Specifies the `prometheus` label on the created PodMonitor. This is used when different Prometheus instances have label selectors matching different PodMonitors.
#### **metrics.podmonitor.interval** ~ `string`
> Default value:
> ```yaml
> 60s
> ```

The interval to scrape metrics.
#### **metrics.podmonitor.scrapeTimeout** ~ `string`
> Default value:
> ```yaml
> 30s
> ```

The timeout before a metrics scrape fails.
#### **metrics.podmonitor.labels** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Additional labels to add to the PodMonitor.
#### **metrics.podmonitor.annotations** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Additional annotations to add to the PodMonitor.
#### **metrics.podmonitor.honorLabels** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Keep labels from scraped data, overriding server-side labels.
#### **metrics.podmonitor.endpointAdditionalProperties** ~ `object`
> Default value:
> ```yaml
> {}
> ```

EndpointAdditionalProperties allows setting additional properties on the endpoint such as relabelings, metricRelabelings etc.  
  
For example:

```yaml
endpointAdditionalProperties:
 relabelings:
 - action: replace
   sourceLabels:
   - __meta_kubernetes_pod_node_name
   targetLabel: instance
```
#### **replicaCount** ~ `number`
> Default value:
> ```yaml
> 1
> ```

default replicas, do not scale up
#### **image.repository** ~ `string`
> Default value:
> ```yaml
> registry.venafi.cloud/venafi-agent/venafi-agent
> ```

The container image for the Venafi Enhanced Issuer manager.
#### **image.pullPolicy** ~ `string`
> Default value:
> ```yaml
> IfNotPresent
> ```

Kubernetes imagePullPolicy on Deployment.
#### **image.tag** ~ `string`
> Default value:
> ```yaml
> v0.0.0
> ```

Overrides the image tag whose default is the chart appVersion.
#### **imagePullSecrets** ~ `array`
> Default value:
> ```yaml
> []
> ```

Specify image pull credentials if using a private registry. Example:  
 - name: my-pull-secret
#### **nameOverride** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Helm default setting to override release name, usually leave blank.
#### **fullnameOverride** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Helm default setting, use this to shorten the full install name.
#### **serviceAccount.create** ~ `bool`
> Default value:
> ```yaml
> true
> ```

Specifies whether a service account should be created.
#### **serviceAccount.annotations** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Annotations YAML to add to the service account.
#### **serviceAccount.name** ~ `string`
> Default value:
> ```yaml
> ""
> ```

The name of the service account to use. If blank and `serviceAccount.create` is true, a name is generated using the fullname template of the release.
#### **podAnnotations** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Additional YAML annotations to add the the pod.
#### **podSecurityContext** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Optional Pod (all containers) `SecurityContext` options, see https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod.  
  
Example:  
  
 podSecurityContext

```yaml
runAsUser: 1000
runAsGroup: 3000
fsGroup: 2000
```
#### **http_proxy** ~ `string`

Configures the HTTP_PROXY environment variable where a HTTP proxy is required.

#### **https_proxy** ~ `string`

Configures the HTTPS_PROXY environment variable where a HTTP proxy is required.

#### **no_proxy** ~ `string`

Configures the NO_PROXY environment variable where a HTTP proxy is required, but certain domains should be excluded.

#### **securityContext.capabilities.drop[0]** ~ `string`
> Default value:
> ```yaml
> ALL
> ```
#### **securityContext.readOnlyRootFilesystem** ~ `bool`
> Default value:
> ```yaml
> true
> ```
#### **securityContext.runAsNonRoot** ~ `bool`
> Default value:
> ```yaml
> true
> ```
#### **resources.requests.memory** ~ `string`
> Default value:
> ```yaml
> 200Mi
> ```
#### **resources.requests.cpu** ~ `string`
> Default value:
> ```yaml
> 200m
> ```
#### **resources.limits.memory** ~ `string`
> Default value:
> ```yaml
> 500Mi
> ```
#### **nodeSelector** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Embed YAML for nodeSelector settings, see  
https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes/
#### **tolerations** ~ `array`
> Default value:
> ```yaml
> []
> ```

Embed YAML for toleration settings, see  
https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
#### **affinity** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Embed YAML for Node affinity settings, see  
https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/.
#### **command** ~ `array`
> Default value:
> ```yaml
> []
> ```

Specify the command to run overriding default binary.
#### **extraArgs** ~ `array`
> Default value:
> ```yaml
> []
> ```

Specify additional arguments to pass to the agent binary.  
Example: `["--strict", "--oneshot"]`
#### **volumes** ~ `array`
> Default value:
> ```yaml
> []
> ```

Additional volumes to add to the Venafi Kubernetes Agent container. This is useful for mounting a custom CA bundle. For example:

```yaml
volumes:
  - name: cabundle
    configMap:
      name: cabundle
      optional: false
      defaultMode: 0644
```

In order to create the ConfigMap, you can use the following command:  
  
    kubectl create configmap cabundle \  
      --from-file=cabundle=./your/custom/ca/bundle.pem
#### **volumeMounts** ~ `array`
> Default value:
> ```yaml
> []
> ```

Additional volume mounts to add to the Venafi Kubernetes Agent container. This is useful for mounting a custom CA bundle. Any PEM certificate mounted under /etc/ssl/certs will be loaded by the Venafi Kubernetes Agent. For

```yaml
example:
```



```yaml
volumeMounts:
  - name: cabundle
    mountPath: /etc/ssl/certs/cabundle
    subPath: cabundle
    readOnly: true
```
#### **authentication.secretName** ~ `string`
> Default value:
> ```yaml
> agent-credentials
> ```

Name of the secret containing the private key
#### **authentication.secretKey** ~ `string`
> Default value:
> ```yaml
> privatekey.pem
> ```

Key name in the referenced secret
### Venafi Connection


Configure VenafiConnection authentication
#### **authentication.venafiConnection.enabled** ~ `bool`
> Default value:
> ```yaml
> false
> ```

When set to true, the Venafi Kubernetes Agent will authenticate to. Venafi using the configuration in a VenafiConnection resource. Use `venafiConnection.enabled=true` for [secretless authentication](https://docs.venafi.cloud/vaas/k8s-components/t-install-tlspk-agent/). When set to true, the `authentication.secret` values will be ignored and the. Secret with `authentication.secretName` will _not_ be mounted into the  
Venafi Kubernetes Agent Pod.
#### **authentication.venafiConnection.name** ~ `string`
> Default value:
> ```yaml
> venafi-components
> ```

The name of a VenafiConnection resource which contains the configuration for authenticating to Venafi.
#### **authentication.venafiConnection.namespace** ~ `string`
> Default value:
> ```yaml
> venafi
> ```

The namespace of a VenafiConnection resource which contains the configuration for authenticating to Venafi.
#### **config.server** ~ `string`
> Default value:
> ```yaml
> https://api.venafi.cloud/
> ```

API URL of the Venafi Control Plane API. For EU tenants, set this value to https://api.venafi.eu/. If you are using the VenafiConnection authentication method, you must set the API URL using the field `spec.vcp.url` on the  
VenafiConnection resource instead.
#### **config.clientId** ~ `string`
> Default value:
> ```yaml
> ""
> ```

The client-id to be used for authenticating with the Venafi Control. Plane. Only useful when using a Key Pair Service Account in the Venafi. Control Plane. You can obtain the cliend ID by creating a Key Pair Service  
Account in the Venafi Control Plane.
#### **config.period** ~ `string`
> Default value:
> ```yaml
> 0h1m0s
> ```

Send data back to the platform every minute unless changed.
#### **config.clusterName** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Name for the cluster resource if it needs to be created in Venafi Control  
Plane.
#### **config.clusterDescription** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Description for the cluster resource if it needs to be created in Venafi  
Control Plane.
#### **config.ignoredSecretTypes[0]** ~ `string`
> Default value:
> ```yaml
> kubernetes.io/service-account-token
> ```
#### **config.ignoredSecretTypes[1]** ~ `string`
> Default value:
> ```yaml
> kubernetes.io/dockercfg
> ```
#### **config.ignoredSecretTypes[2]** ~ `string`
> Default value:
> ```yaml
> kubernetes.io/dockerconfigjson
> ```
#### **config.ignoredSecretTypes[3]** ~ `string`
> Default value:
> ```yaml
> kubernetes.io/basic-auth
> ```
#### **config.ignoredSecretTypes[4]** ~ `string`
> Default value:
> ```yaml
> kubernetes.io/ssh-auth
> ```
#### **config.ignoredSecretTypes[5]** ~ `string`
> Default value:
> ```yaml
> bootstrap.kubernetes.io/token
> ```
#### **config.ignoredSecretTypes[6]** ~ `string`
> Default value:
> ```yaml
> helm.sh/release.v1
> ```
#### **config.configmap.name** ~ `unknown`
> Default value:
> ```yaml
> null
> ```
#### **config.configmap.key** ~ `unknown`
> Default value:
> ```yaml
> null
> ```
#### **podDisruptionBudget.enabled** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Enable or disable the PodDisruptionBudget resource, which helps prevent downtime during voluntary disruptions such as during a Node upgrade.
### CRDs


The CRDs installed by this chart are annotated with "helm.sh/resource-policy: keep", this prevents them from being accidentally removed by Helm when this chart is deleted. After deleting the installed chart, the user still has to manually remove the remaining CRDs.
#### **crds.forceRemoveValidationAnnotations** ~ `bool`
> Default value:
> ```yaml
> false
> ```

The 'x-kubernetes-validations' annotation is not supported in Kubernetes 1.22 and below. This annotation is used by CEL, which is a feature introduced in Kubernetes 1.25 that improves how validation is performed. This option allows to force the 'x-kubernetes-validations' annotation to be excluded, even on Kubernetes 1.25+ clusters.
#### **crds.keep** ~ `bool`
> Default value:
> ```yaml
> false
> ```

This option makes it so that the "helm.sh/resource-policy": keep annotation is added to the CRD. This will prevent Helm from uninstalling the CRD when the Helm release is uninstalled.
#### **crds.venafiConnection.include** ~ `bool`
> Default value:
> ```yaml
> false
> ```

When set to false, the rendered output does not contain the. VenafiConnection CRDs and RBAC. This is useful for when the. Venafi Connection resources are already installed separately.
#### **podDisruptionBudget.enabled** ~ `bool`
> Default value:
> ```yaml
> false
> ```
#### **podDisruptionBudget.minAvailable** ~ `number`
> Default value:
> ```yaml
> 1
> ```
#### **podDisruptionBudget.maxUnavailable** ~ `number`
> Default value:
> ```yaml
> 1
> ```

<!-- /AUTO-GENERATED -->