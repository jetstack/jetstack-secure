# disco-agent

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

The agent supports **two authentication methods**, selected automatically by
config:

| Set this | Method used |
|---|---|
| `config.cyberark.serviceId` (Conjur authn-jwt service-id) | **Conjur JWT exchange** — exchanges a projected ServiceAccount token for a short-lived Conjur access token. No stored password. Preferred for new installs. |
| `ARK_USERNAME` + `ARK_SECRET` in the Secret (and no `serviceId`) | **Legacy CyberArk Identity username/password** — backward compatible with existing GA installs. |

If **both** are set, the Conjur `serviceId` wins (so a migrating install can add
the service-id before removing its old credentials) and a warning is logged. If
**neither** is set, the agent fails closed at startup.

The only credential always required in the Kubernetes Secret is the CyberArk
tenant subdomain (`ARK_SUBDOMAIN`).

```sh
export ARK_SUBDOMAIN=      # your CyberArk tenant subdomain, e.g. tlskp-test
# OPTIONAL: Discovery API URL for non-production environments
export ARK_DISCOVERY_API=https://platform-discovery.integration-cyberark.cloud/
```

Create the Secret:

```sh
kubectl create secret generic agent-credentials \
        --namespace "$NAMESPACE" \
        --from-literal=ARK_SUBDOMAIN=$ARK_SUBDOMAIN
# Add the optional key only if targeting a non-production Discovery API:
# kubectl patch secret agent-credentials -n "$NAMESPACE" \
#   --type=json -p '[{"op":"add","path":"/data/ARK_DISCOVERY_API","value":"'"$(echo -n $ARK_DISCOVERY_API | base64)"'"}]'
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
  ARK_SUBDOMAIN: "tlskp-test"   # your CyberArk tenant subdomain
  # OPTIONAL: uncomment for non-production Discovery API
  # ARK_DISCOVERY_API: https://platform-discovery.integration-cyberark.cloud/
  # LEGACY (only if NOT using Conjur serviceId) — username/password auth:
  # ARK_USERNAME: "svc-agent@tenant"
  # ARK_SECRET: "<password>"
```

### Configure Conjur JWT authentication

> Skip this section if you are using the legacy username/password method
> (set `ARK_USERNAME`/`ARK_SECRET` in the Secret and leave `serviceId` empty).

Set `config.cyberark.serviceId` to the authn-jwt authenticator service ID
configured for this cluster in your Conjur tenant. This is the **bare service-id
segment** (e.g. `disco-agent`), NOT the policy path `conjur/authn-jwt/disco-agent`
— the agent builds the authenticate URL as
`<base>/authn-jwt/<serviceId>/<account>/authenticate`, so a path here would
double the `conjur/authn-jwt` prefix. The remaining defaults are correct for
CyberArk-hosted tenants:

| Value | Default | Description |
|---|---|---|
| `config.cyberark.serviceId` | `""` | Conjur authn-jwt service ID (required). Example: `conjur/authn-jwt/disco-agent` |
| `config.cyberark.account` | `conjur` | Conjur account name. Always `conjur` for CyberArk-hosted tenants. |
| `config.cyberark.jwtSource` | `file` | Token source. `file` = projected SA-token volume (default). `spiffe` deferred. |
| `config.cyberark.jwtFilePath` | `/var/run/secrets/tokens/jwt` | Path to the projected token file. Auto-mounted by the chart when `jwtSource=file`. |

The chart automatically renders a projected ServiceAccount token volume
(audience=`conjur`, expiry 600 s) and mounts it at `/var/run/secrets/tokens`
when `config.cyberark.jwtSource` is `file` (the default). No manual volume
configuration is required.

### Per-tenant Conjur onboarding

Before deploying the agent against a new tenant, complete the following steps
in the Conjur tenant:

1. **Enable the authn-jwt authenticator** with `audience=conjur` and
   `token-app-property=sub`.

   ```yaml
   # conjur-authn-jwt-policy.yml
   - !policy
     id: conjur/authn-jwt/disco-agent
     body:
       - !webservice

       - !variable jwks-uri
       - !variable token-app-property
       - !variable issuer
       - !variable audience

       - !group hosts
       - !permit
         role: !group hosts
         privilege: [ read, authenticate ]
         resource: !webservice
   ```

2. **Set the authenticator variables** (values shown as examples):

   ```sh
   conjur variable set -i conjur/authn-jwt/disco-agent/token-app-property -v sub
   conjur variable set -i conjur/authn-jwt/disco-agent/audience           -v conjur
   conjur variable set -i conjur/authn-jwt/disco-agent/issuer             -v https://kubernetes.default.svc.cluster.local
   conjur variable set -i conjur/authn-jwt/disco-agent/jwks-uri           -v https://kubernetes.default.svc.cluster.local/openid/v1/jwks
   ```

3. **Pre-create a Conjur host** for the agent ServiceAccount. The `id` must
   match the Kubernetes ServiceAccount's `sub` claim
   (`system:serviceaccount:<namespace>:<sa-name>`):

   ```yaml
   # conjur-agent-host-policy.yml
   - !host
     id: system:serviceaccount/cyberark/disco-agent
     annotations:
       authn-jwt/disco-agent/sub: system:serviceaccount/cyberark/disco-agent
   ```

4. **Add the host to the `data/disco/snapshot-uploaders` group** so the
   authorizer grants it upload access:

   ```yaml
   - !grant
     role: !group data/disco/snapshot-uploaders
     member: !host system:serviceaccount/cyberark/disco-agent
   ```

5. **Add the host to the authn-jwt authenticator's hosts group**:

   ```yaml
   - !grant
     role: !group conjur/authn-jwt/disco-agent/hosts
     member: !host system:serviceaccount/cyberark/disco-agent
   ```

### Deploy the agent

```sh
helm upgrade agent "oci://${OCI_BASE}/charts/disco-agent" \
     --install \
     --create-namespace \
     --namespace "$NAMESPACE" \
     --set fullnameOverride=disco-agent \
     --set config.cyberark.serviceId=disco-agent \
     --set acceptTerms=true
```

### Troubleshooting

Check the Pod and its events:
```sh
kubectl describe -n cyberark pods -l app.kubernetes.io/name=disco-agent
```

Check the logs:
```sh
kubectl logs deployments/disco-agent --namespace "${NAMESPACE}" --follow
```

#### Conjur authentication errors

| Symptom | Likely cause | Fix |
|---|---|---|
| Agent logs `401 Unauthorized` from Conjur | ServiceAccount token `audience` does not match the authenticator's configured `audience` value, or the authn-jwt authenticator is not enabled for the account | Confirm `audience=conjur` in both the projected volume (chart default) and the Conjur `conjur/authn-jwt/<serviceId>/audience` variable; ensure the authenticator is enabled (`CONJUR_AUTHENTICATORS` includes `authn-jwt/<serviceId>`) |
| Agent logs `403 Forbidden` from the upload API | The agent's Conjur host is not a member of `data/disco/snapshot-uploaders` | Add the host to the group per step 4 of the onboarding runbook above |
| Agent logs `500` / no upload attempt | Conjur is unreachable or returned an unexpected error | Check network policy / DNS; inspect Conjur audit logs for the host identity |

## Values

<!-- AUTO-GENERATED -->

#### **replicaCount** ~ `number`
> Default value:
> ```yaml
> 1
> ```

This will set the replicaset count more information can be found here: https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/
#### **acceptTerms** ~ `bool`
> Default value:
> ```yaml
> false
> ```

Must be set to indicate that you have read and accepted the CyberArk Terms of Service. If false, the helm chart will fail to install and will print a message with instructions on how to accept the TOS.
#### **imageRegistry** ~ `string`
> Default value:
> ```yaml
> quay.io
> ```

The container registry used for disco-agent images by default. This can include path prefixes (e.g. "artifactory.example.com/docker").

#### **imageNamespace** ~ `string`
> Default value:
> ```yaml
> jetstack
> ```

The repository namespace used for disco-agent images by default.  
Examples:  
- jetstack  
- custom-namespace

#### **image.registry** ~ `string`

Deprecated: per-component registry prefix.  
  
If set, this value is *prepended* to the image repository that the chart would otherwise render. This applies both when `image.repository` is set and when the repository is computed from  
`imageRegistry` + `imageNamespace` + `image.name`.  
  
This can produce "double registry" style references such as  
`legacy.example.io/quay.io/jetstack/...`. Prefer using the global  
`imageRegistry`/`imageNamespace` values.

#### **image.repository** ~ `string`
> Default value:
> ```yaml
> ""
> ```

Full repository override (takes precedence over `imageRegistry`, `imageNamespace`, and `image.name`).  
Example: quay.io/jetstack/disco-agent

#### **image.name** ~ `string`
> Default value:
> ```yaml
> disco-agent
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

#### **config.period** ~ `string`
> Default value:
> ```yaml
> 12h0m0s
> ```

Push data every 12 hours unless changed.
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
#### **config.clusterName** ~ `string`
> Default value:
> ```yaml
> ""
> ```

A human readable name for the cluster where the agent is deployed (optional).  
  
This cluster name will be associated with the data that the agent uploads to the Discovery and Context service. If empty (the default), the service account name will be used instead.
#### **config.clusterDescription** ~ `string`
> Default value:
> ```yaml
> ""
> ```

A short description of the cluster where the agent is deployed (optional).  
  
This description will be associated with the data that the agent uploads to the Discovery and Context service. The description may include contact information such as the email address of the cluster administrator, so that any problems and risks identified by the Discovery and Context service can be communicated to the people responsible for the affected secrets.
#### **config.sendSecretValues** ~ `bool`
> Default value:
> ```yaml
> true
> ```

Enable sending of Secret values to CyberArk in addition to metadata. Metadata is always sent, but the actual values of Secrets are not sent by default. When enabled, Secret data is encrypted using envelope encryption using a key managed by CyberArk, fetched from the Discovery and Context service.
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
Defaults to the disco-agent namespace.

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

