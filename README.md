<p align="center">
<a href="https://prow.build-infra.jetstack.net/?job=post-preflight-release-canary">
<!-- prow build badge, godoc, and go report card-->
<img alt="Build Status" src="https://prow.build-infra.jetstack.net/badge.svg?jobs=post-preflight-release-canary">
</a>
<a href="https://godoc.org/github.com/jetstack/preflight"><img src="https://godoc.org/github.com/jetstack/preflight?status.svg"></a>
<a href="https://goreportcard.com/report/github.com/jetstack/preflight"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/jetstack/preflight" /></a>
</p>

# Jetstack Preflight

Preflight is a tool to automatically perform Kubernetes cluster configuration
checks using [Open Policy Agent (OPA)](https://www.openpolicyagent.org/).

<!-- markdown-toc start - Don't edit this section. Run M-x
markdown-toc-refresh-toc -->

**Table of Contents**

* [Jetstack Preflight](#jetstack-preflight)
   * [Project Background](#project-background)
   * [Agent](#agent)
   * [Packages](#packages)
   * [Installation](#installation)

<!-- markdown-toc end -->

## Project Background

Preflight was originally designed to automate Jetstack's production readiness
assessments.
These are consulting sessions in which a Jetstack engineer inspects a customer's
cluster to suggest improvements and identify configuration issues.
The product of this assessment is a report
which describes any problems and offers remediation advice.

While these assessments have provided a lot of value to many customers, with a
complex system like Kubernetes it's hard to thoroughly check everything.
Automating the checks allows them to be more comprehensive and much faster.

The automation also allows the checks to be run repeatedly, meaning they can be
deployed in-cluster to provide continuous configuration checking. This enables
new interesting use cases as policy compliance audits.

## Agent

The Preflight _agent_ uses _data gatherers_ to collect required data from
Kubernetes and cloud provider APIs before formatting it as JSON for analysis.
Once data has been collected, it is sent to the configured backend.

To run the Agent locally you can run:

```bash
preflight agent --agent-config-file ./path/to/agent/config/file.yaml
```

Or, to build and run a version from master:

```bash
go run main.go agent --agent-config-file ./path/to/agent/config/file.yaml
```

You can find the example agent file
[here](https://github.com/jetstack/preflight/blob/master/agent.yaml).

You might also want to run a local echo server to monitor requests the agent
sends:

```bash
go run main.go echo
```

## Packages

Policies for cluster configuration are encoded into *Preflight packages*.  Each
package focuses on a different infrastructure component, for example the `gke`
package provides rules for the configuration of a GKE cluster.

Preflight packages are implemented using
[Open Policy Agent](https://www.openpolicyagent.org) with evaluation
taking place in the SaaS backend.

## Installation

The following instructions walk through the installation of the Preflight agent
to gather data about cluster pods and send them to the backend for analysis.

**To complete the secret manifest below, you will need to have a Preflight
token.**

First create a namespace for the preflight components:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: preflight
```

Next create a secret like the following, substituting your token:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: agent-config
  namespace: preflight
type: Opaque
stringData:
  config.yaml: |
    schedule: "* * * * *"
    token: # enter your token here
    endpoint:
      protocol: https
      host: preflight.jetstack.io
      path: /api/v1/datareadings
    data-gatherers:
    - name: "pods"
      kind: "k8s"
      config:
        resource-type:
          resource: pods
          version: v1
```

Now create a service account with permissions to read cluster resources:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: agent
  namespace: preflight
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: preflight-agent-cluster-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  # will be able to view all resources, but not rbac and secrets
  name: view
subjects:
- kind: ServiceAccount
  name: agent
  namespace: preflight
```

Finally deploy the agent:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent
  namespace: preflight
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: agent
  template:
    metadata:
      labels:
        app.kubernetes.io/name: agent
    spec:
      serviceAccountName: agent
      volumes:
      - name: config
        secret:
          secretName: agent-config
      containers:
      - name: agent
        image: quay.io/jetstack/preflight:7d4fa467258b7592d68fd660f1fd1d42e7332231
        args:
        - "agent"
        - "-c"
        - "/etc/secrets/preflight/agent/config.yaml"
        volumeMounts:
        - name: config
          mountPath: "/etc/secrets/preflight/agent"
          readOnly: true
        resources:
          requests:
            memory: "200Mi"
            cpu: "200m"
          limits:
            memory: "200Mi"
            cpu: "200m"
```
