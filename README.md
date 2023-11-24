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
for the agent before getting started.

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

## Release Process

The release process is semi-automated.
It starts with the following manual steps:

1. Choose the next semver version number.
   This project has only ever incremented the "patch" number (never the "minor" number) regardless of the scope of the changes.
1. Create a branch.
1. Increment version numbers in the `venafi-kubernetes-agent` Helm chart.
   (the `jetstack-secure` Helm chart uses a different version scheme and is updated and released separately):
   1. Increment the `version` value in [Chart.yaml](deploy/charts/venafi-kubernetes-agent/Chart.yaml).
      DO NOT use a `v` prefix.
      The `v` prefix [breaks Helm OCI operations](https://github.com/helm/helm/issues/11107).
   1. Increment `appVersion` value in [Chart.yaml](deploy/charts/venafi-kubernetes-agent/Chart.yaml).
      Use a `v` prefix, to match the Docker image tag.
   1. Increment the `image.tag` value in [values.yaml](deploy/charts/venafi-kubernetes-agent/values.yaml).
      Use a `v` prefix.
   1. Commit the changes.
1. Create a pull request and wait for it to be approved.
1. Merge the branch.
1. Push a semver tag with a `v` prefix: `vX.Y.Z`.

This will trigger the following automated processes:

1. Two Docker images are built and pushed to a public `quay.io` registry, by the [release-master workflow](.github/workflows/release-master.yml):
   * `quay.io/jetstack/preflight`: is pulled directly by tier 1 Jetstack Secure users, who do not have access to the Jetstack Enterprise Registry.
   * `quay.io/jetstack/venafi-agent`: is mirrored to a public Venafi OCI registry for Venafi TLS Protect for Kubernetes users.

2. The Docker images are mirrored by private Venafi CI pipelines, to:
   * [Jetstack Enterprise Registry](https://platform.jetstack.io/documentation/installation/agent#1-obtain-oci-registry-credentials):
     for Tier 2 Jetstack Secure users. Tier 2 grants users access to this registry.
   * [Venafi private Registry](https://docs.venafi.cloud/vaas/k8s-components/th-guide-confg-access-to-tlspk-enterprise-components/):
     for Tier 2 Venafi TLS Protect for Kubernetes users. Tier 2 grants users access to this registry.
   * [Venafi public Registry](https://registry.venafi.cloud/public/venafi-images/venafi-kubernetes-agent):
     for Tier 1 Venafi TLS Protect for Kubernetes users. Tier 1 users do not have access to the private registry. (TODO)

### Helm Chart: venafi-kubernetes-agent

The [venafi-kubernetes-agent](deploy/charts/venafi-kubernetes-agent/README.md) chart
is released manually, as follows:

```sh
export VERSION=0.1.43
helm package deploy/charts/venafi-kubernetes-agent --version "${VERSION}"
helm push venafi-kubernetes-agent-${VERSION}.tgz oci://eu.gcr.io/jetstack-secure-enterprise/charts
```

> ℹ️ To test the Helm chart before releasing it, use a [pre-release suffix](https://semver.org/#spec-item-9). E.g.
> `export VERSION=0.1.43-alpha.0`.

The chart will be mirrored to:
 * `registry.venafi.cloud/charts/venafi-kubernetes-agent` (Public)
 * `private-registry.venafi.cloud/charts/venafi-kubernetes-agent` (Private, US)
 * `private-registry.venafi.eu/charts/venafi-kubernetes-agent` (Private, EU)

### Helm Chart: jetstack-agent

The [jetstack-agent](deploy/charts/jetstack-agent/README.md) chart has a different version number to the agent.
This is because the first version of *this* chart was given version `0.1.0`,
while the app version at the time was `0.1.38`.
And this allows the chart to be updated and released more frequently than the Docker image if necessary.
This chart is for [Jetstack Secure](https://platform.jetstack.io/documentation/installation/agent#jetstack-agent-helm-chart-installation).

1. Create a branch
1. Increment version numbers.
   1. Increment the `version` value in [Chart.yaml](deploy/charts/jetstack-agent/Chart.yaml).
      DO NOT use a `v` prefix.
      The `v` prefix [breaks Helm OCI operations](https://github.com/helm/helm/issues/11107).
   1. Increment the `appVersion` value in [Chart.yaml](deploy/charts/jetstack-agent/Chart.yaml).
      Use a `v` prefix, to match the Docker image tag.
   1. Increment the `image.tag` value in [values.yaml](deploy/charts/jetstack-agent/values.yaml).
      Use a `v` prefix, to match the Docker image tag.
1. Create a pull request and wait for it to be approved.
1. Merge the branch
1. Push a tag, using the format: `chart-vX.Y.Z`.
   This unique tag format is recognized by the private CI pipeline that builds and publishes the chart.

The chart will be published to
the [Jetstack Enterprise Registry](https://platform.jetstack.io/documentation/installation/agent#1-obtain-oci-registry-credentials)
by a private CI pipeline managed by Venafi.
