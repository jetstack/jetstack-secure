# Venafi Kubernetes Agent

[![tests](https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml/badge.svg?branch=master&event=push)](https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jetstack/jetstack-secure.svg)](https://pkg.go.dev/github.com/jetstack/jetstack-secure)
[![Go Report Card](https://goreportcard.com/badge/github.com/jetstack/jetstack-secure)](https://goreportcard.com/report/github.com/jetstack/jetstack-secure)

"The agent" manages your machine identities across Cloud Native Kubernetes and OpenShift environments and builds a detailed view of the enterprise security posture.

## Installation

Please [review the documentation](https://docs.venafi.cloud/vaas/k8s-components/c-tlspk-agent-overview/) for the agent.

Detailed installation instructions are available for a variety of methods.

## Local Execution

To build and run a version from master:

```bash
go run main.go agent --agent-config-file ./path/to/agent/config/file.yaml -p 0h1m0s
```

You can configure the agent to perform one data gathering loop and output the data to a local file:

```bash
go run . agent \
   --agent-config-file examples/one-shot-secret.yaml \
   --one-shot \
   --output-path output.json
```

> Some examples of agent configuration files:
> - [./agent.yaml](./agent.yaml).
> - [./examples/one-shot-secret.yaml](./examples/one-shot-secret.yaml).
> - [./examples/cert-manager-agent.yaml](./examples/cert-manager-agent.yaml).

You might also want to run a local echo server to monitor requests sent by the agent:

```bash
go run main.go echo
```

## Metrics

The agent exposes its metrics through a Prometheus server, on port 8081.

The Prometheus server is disabled by default but can be enabled by passing the `--enable-metrics` flag to the agent binary.

If you deploy the agent using the venafi-kubernetes-agent Helm chart, the metrics server will be enabled by default, on port 8081.

If you use the Prometheus Operator, you can use `--set metrics.podmonitor.enabled=true` to deploy a `PodMonitor` resource,
which will add the venafi-kubernetes-agent metrics to your Prometheus server.

The following metrics are collected:

- Go collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus `client_golang`.
- Process collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus `client_golang`.
- Agent metrics: `data_readings_upload_size`: Data readings upload size (in bytes) sent by the in-cluster agent.
