# cyberark-disco-agent

The Cyberark Discovery and Context Agent connects your Kubernetes or OpenShift
cluster to the Discovery and Context service of the CyberArk Identity Security Platform.

## Quick Start

### Create a Namespace

Create a namespace for the agent:

```sh
export NAMESPACE=cyberark
kubectl create ns "$NAMESPACE" || true
```

### Add credentials to a Secret

You will require tenant details and credentials for the CyberArk Identity Security Platform.
Put them in the following environment variables:

```sh
export ARK_SUBDOMAIN=      # your CyberArk tenant subdomain e.g. tlskp-test
export ARK_USERNAME=       # your CyberArk username
export ARK_SECRET=         # your CyberArk password
# OPTIONAL: the URL for the CyberArk Discovery API if not using the production environment
export ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/
```

Create a Secret containing the tenant details and credentials:

```sh
kubectl create secret generic agent-credentials \
        --namespace "$NAMESPACE" \
        --from-literal=ARK_USERNAME=$ARK_USERNAME \
        --from-literal=ARK_SECRET=$ARK_SECRET \
        --from-literal=ARK_SUBDOMAIN=$ARK_SUBDOMAIN \
        --from-literal=ARK_DISCOVERY_API=$ARK_DISCOVERY_API
```

Alternatively, use the following Secret as a template:

```yaml
# agent-credentials.yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-credentials
  namespace: cyberark
type: Opaque
stringData:
  ARK_SUBDOMAIN: $ARK_SUBDOMAIN # your CyberArk tenant subdomain e.g. tlskp-test
  ARK_SECRET: $ARK_SECRET       # your CyberArk password
  ARK_USERNAME: $ARK_USERNAME   # your CyberArk username
  # OPTIONAL: the URL for the CyberArk Discovery API if not using the production environment
  # ARK_DISCOVERY_API: https://platform-discovery.integration-cyberark.cloud/
```

### Deploy the agent

Deploy the agent:

```sh
helm upgrade agent "oci://${OCI_BASE}/charts/cyberark-disco-agent" \
     --install \
     --create-namespace \
     --namespace "$NAMESPACE" \
     --set fullnameOverride=disco-agent
```

### Troubleshooting

Check the Pod and its events:
```sh
kubectl describe -n cyberark pods -l app.kubernetes.io/name=cyberark-disco-agent
```

Check the logs:
```sh
kubectl logs deployments/disco-agent --namespace "${NAMESPACE}" --follow
```

## Values

<!-- AUTO-GENERATED -->

#### **replicaCount** ~ `number`
> Default value:
> ```yaml
> 1
> ```

This will set the replicaset count more information can be found here: https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/
#### **image.repository** ~ `string`
> Default value:
> ```yaml
> ""
> ```
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

Overrides the image tag whose default is the chart appVersion.
#### **image.digest** ~ `string`
> Default value:
> ```yaml
> ""
> ```

The image digest
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

#### **config.period** ~ `string`
> Default value:
> ```yaml
> 1h0m0s
> ```

Push data every hour unless changed.
#### **config.excludeAnnotationKeysRegex** ~ `array`
> Default value:
> ```yaml
> []
> ```

You can configure the agent to exclude some annotations or labels from being pushed . All Kubernetes objects are affected. The objects are still pushed, but the specified annotations and labels are removed before being pushed.  
  
Dots is the only character that needs to be escaped in the regex. Use either double quotes with escaped single quotes or unquoted strings for the regex to avoid YAML parsing issues with `\.`.  
  
Example: excludeAnnotationKeysRegex: ['^kapp\.k14s\.io/original.*']
#### **config.excludeLabelKeysRegex** ~ `array`
> Default value:
> ```yaml
> []
> ```
#### **authentication.secretName** ~ `string`
> Default value:
> ```yaml
> agent-credentials
> ```
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
Defaults to the cyberark-disco-agent namespace.

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

