[![release-master](https://github.com/jetstack/jetstack-secure/actions/workflows/release-master.yml/badge.svg)](https://github.com/jetstack/jetstack-secure/actions/workflows/release-master.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jetstack/jetstack-secure.svg)](https://pkg.go.dev/github.com/jetstack/jetstack-secure)
[![Go Report Card](https://goreportcard.com/badge/github.com/jetstack/jetstack-secure)](https://goreportcard.com/report/github.com/jetstack/jetstack-secure)

![Jetstack Secure](./docs/images/js.png)

[Jetstack Secure](https://www.jetstack.io/jetstack-secure/) manages your machine identities across Cloud Native Kubernetes and OpenShift environments and builds a detailed view of the enterprise security posture.

This repo contains the open source in-cluster agent of Jetstack Secure, that sends data to the [Jetstack Secure
SaaS](https://platform.jetstack.io).

> **Wondering about Preflight?** Preflight was the name for the project that was the foundation for the Jetstack Secure platform. It was a tool to perform configuration checks on a Kubernetes cluster using OPA's REGO policy. We decided to incorporate that functionality as part of the Jetstack Secure SaaS service, making this component a basic agent. You can find the old Preflight Check functionality in the git history ( tagged as `preflight-local-check` and you also check [this documentation](https://github.com/jetstack/jetstack-secure/blob/preflight-local-check/docs/check.md).

## Installation

Please [review the documentation](https://platform.jetstack.io/documentation/installation/agent)
for the agent to get started.

The released container images are cryptographically signed by
[`cosign`](https://github.com/sigstore/cosign), with
[SLSA provenance](https://slsa.dev/provenance/v0.2) and a
[CycloneDX SBOM](https://cyclonedx.org/) attached. For instructions on how to
verify those signatures and attachments, refer to
[this guide](docs/guides/cosign).

## Local Execution

To build and run a version from master:

```bash
go run main.go agent --agent-config-file ./path/to/agent/config/file.yaml -p 0h1m0s
```

You can find the example agent file
[here](https://github.com/jetstack/preflight/blob/master/agent.yaml).

You might also want to run a local echo server to monitor requests the agent
sends:

```bash
go run main.go echo
```

## Metrics

The Jetstack-Secure agent exposes its metrics through a Prometheus server, on port 8081.
The Prometheus server is disabled by default but can be enabled by passing the `--enable-metrics` flag to the agent binary.
