# discovery-agent

The Discovery Agent connects your Kubernetes or OpenShift cluster to Palo Alto NGTS.

## Values

<!-- AUTO-GENERATED -->

#### **config.tsgID** ~ `number,string`
> Default value:
> ```yaml
> ""
> ```

Required: The TSG (Tenant Service Group) ID to use when connecting to SCM.


#### **config.clusterName** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Required: A human readable name for the cluster into which the agent is being deployed.  
  
This cluster name will be associated with the data that the agent uploads to the backend.

#### **config.clusterDescription** ~ `string`
> Default value:
> ```yaml
> ""
> ```

A short description of the cluster where the agent is deployed (optional).  
  
This description will be associated with the data that the agent uploads to the backend.

#### **config.claimableCerts** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Whether discovered certs can be claimed by other tenants (optional). true = certs are left unassigned, available for any tenant to claim. false (default) = certs are owned by this cluster's tenant.
#### **config.period** ~ `string`
> Default value:
> ```yaml
> 0h1m0s
> ```

How often to push data to the remote server

#### **config.excludeAnnotationKeysRegex** ~ `array`
> Default value:
> ```yaml
> []
> ```

You can configure the agent to exclude some annotations or labels from being pushed. All Kubernetes objects are affected. The objects are still pushed, but the specified annotations and labels are removed before being pushed.  
  
Dots is the only character that needs to be escaped in the regex. Use either double quotes with escaped single quotes or unquoted strings for the regex to avoid YAML parsing issues with `\.`.  
  
Example: excludeAnnotationKeysRegex: ['^kapp\.k14s\.io/original.*']
#### **config.excludeLabelKeysRegex** ~ `array`
> Default value:
> ```yaml
> []
> ```
#### **config.clientID** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Deprecated: Client ID for the configured service account. The client ID should be provided in the "clientID" field of the authentication secret (see config.secretName). This field is provided for compatibility for users migrating from the "venafi-kubernetes-agent" chart.

#### **config.secretName** ~ `string`
> Default value:
> ```yaml
> discovery-agent-credentials
> ```

The name of the Secret containing the NGTS built-in service account credentials.  
The Secret must contain the following key:  
- privatekey.pem: PEM-encoded private key for the service account  
The Secret should also contain the following key:  
- clientID:       Service account client ID (config.clientID must be set if not present)

#### **replicaCount** ~ `number`
> Default value:
> ```yaml
> 1
> ```

This will set the replicaset count more information can be found here: https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/
#### **imageRegistry** ~ `string`
> Default value:
> ```yaml
> quay.io
> ```

The container registry used for discovery-agent images by default. This can include path prefixes (e.g. "artifactory.example.com/docker").

#### **imageNamespace** ~ `string`
> Default value:
> ```yaml
> jetstack
> ```

The repository namespace used for discovery-agent images by default.  
Examples:  
- jetstack  
- custom-namespace

#### **image.repository** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Full repository override (takes precedence over `imageRegistry`, `imageNamespace`, and `image.name`).  
Example: quay.io/jetstack/discovery-agent

#### **image.name** ~ `string`
> Default value:
> ```yaml
> discovery-agent
> ```

The image name for the Discovery Agent.  
This is used (together with `imageRegistry` and `imageNamespace`) to construct the full image reference.

#### **image.pullPolicy** ~ `string`
> Default value:
> ```yaml
> IfNotPresent
> ```

This sets the pull policy for images.
#### **image.tag** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Override the image tag to deploy by setting this variable. If no value is set, the chart's appVersion is used.
#### **image.digest** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Override the image digest to deploy by setting this variable. If set together with `image.tag`, the rendered image will include both tag and digest.
#### **imagePullSecrets** ~ `array`
> Default value:
> ```yaml
> []
> ```

This is for the secrets for pulling an image from a private repository more information can be found here: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
#### **nameOverride** ~ `string`
> Default value:
> ```yaml
> ""
> ```

This is to override the chart name.
#### **fullnameOverride** ~ `string`
> Default value:
> ```yaml
> ""
> ```
#### **serviceAccount.create** ~ `bool`
> Default value:
> ```yaml
> true
> ```

Specifies whether a service account should be created
#### **serviceAccount.automount** ~ `bool`
> Default value:
> ```yaml
> true
> ```

Automatically mount a ServiceAccount's API credentials?
#### **serviceAccount.annotations** ~ `object`
> Default value:
> ```yaml
> {}
> ```

Annotations to add to the service account
#### **serviceAccount.name** ~ `string`
> Default value:
> ```yaml
> ""
> ```

The name of the service account to use.  
If not set and create is true, a name is generated using the fullname template
#### **podAnnotations** ~ `object`
> Default value:
> ```yaml
> {}
> ```

This is for setting Kubernetes Annotations to a Pod. For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
#### **podLabels** ~ `object`
> Default value:
> ```yaml
> {}
> ```

This is for setting Kubernetes Labels to a Pod.  
For more information checkout: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
#### **podSecurityContext** ~ `object`
> Default value:
> ```yaml
> {}
> ```
#### **securityContext** ~ `object`
> Default value:
> ```yaml
> allowPrivilegeEscalation: false
> capabilities:
>   drop:
>     - ALL
> readOnlyRootFilesystem: true
> runAsNonRoot: true
> seccompProfile:
>   type: RuntimeDefault
> ```

Add Container specific SecurityContext settings to the container. Takes precedence over `podSecurityContext` when set. See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-capabilities-for-a-container

#### **resources** ~ `object`
> Default value:
> ```yaml
> {}
> ```
#### **volumes** ~ `array`
> Default value:
> ```yaml
> []
> ```

Additional volumes on the output Deployment definition.
#### **volumeMounts** ~ `array`
> Default value:
> ```yaml
> []
> ```

Additional volumeMounts on the output Deployment definition.
#### **nodeSelector** ~ `object`
> Default value:
> ```yaml
> {}
> ```
#### **tolerations** ~ `array`
> Default value:
> ```yaml
> []
> ```
#### **affinity** ~ `object`
> Default value:
> ```yaml
> {}
> ```
#### **http_proxy** ~ `string`

Configures the HTTP_PROXY environment variable where a HTTP proxy is required.

#### **https_proxy** ~ `string`

Configures the HTTPS_PROXY environment variable where a HTTP proxy is required.

#### **no_proxy** ~ `string`

Configures the NO_PROXY environment variable where a HTTP proxy is required, but certain domains should be excluded.

#### **podDisruptionBudget** ~ `object`
> Default value:
> ```yaml
> enabled: false
> ```

Configure a PodDisruptionBudget for the agent's Deployment. If running with multiple replicas, consider setting podDisruptionBudget.enabled to true.

#### **extraArgs** ~ `array`
> Default value:
> ```yaml
> []
> ```

```yaml
extraArgs:
- --logging-format=json
- --log-level=6 # To enable HTTP request logging
```
#### **pprof.enabled** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Enable profiling with the pprof endpoint
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

The namespace that the pod monitor should live in.  
Defaults to the discovery-agent namespace.

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

<!-- /AUTO-GENERATED -->
