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

If you deploy the agent with Helm, using the venafi-kubernetes-agent Helm chart, the metrics server will be enabled by default, on port 8081.
If you use the Prometheus Operator, you can use `--set metrics.podmonitor.enabled=true` to deploy a `PodMonitor` resource,
which will add the venafi-kubernetes-agent metrics to your Prometheus server.

The following metrics are collected:
 * Go collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus client_golang.
 * Process collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus client_golang.
 * Agent metrics:
  * `data_readings_upload_size`: Data readings upload size (in bytes) sent by the jscp in-cluster agent.


## Tiers, Images and Helm Charts

The Docker images are:

|                           Image                           | Access  |                    Tier                     |            Docs             |
|-----------------------------------------------------------|---------|---------------------------------------------|-----------------------------|
| `quay.io/jetstack/preflight`                              | Public  | Tier 1 and 2 of Jetstack Secure             |                             |
| `quay.io/jetstack/venafi-agent`                           | Public  | Not meant for users, used for mirroring     |                             |
| `registry.venafi.cloud/venafi-agent/venafi-agent`         | Public  | Tier 1 of Venafi TLS Protect for Kubernetes |                             |
| `private-registry.venafi.cloud/venafi-agent/venafi-agent` | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][] |
| `private-registry.venafi.eu/venafi-agent/venafi-agent`    | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][] |

[Jetstack Enterprise Registry]: https://platform.jetstack.io/documentation/installation/agent#1-obtain-oci-registry-credentials/
[Venafi Private Registry]: https://docs.venafi.cloud/vaas/k8s-components/th-guide-confg-access-to-tlspk-enterprise-components/

The Helm charts are:

|                              Helm Chart                              | Access  |                    Tier                     |          Documentation           |
|----------------------------------------------------------------------|---------|---------------------------------------------|----------------------------------|
| `oci://eu.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`   | Private | Tier 2 of Jetstack Secure                   | [Jetstack Enterprise Registry][] |
| `oci://us.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`   | Private | Tier 2 of Jetstack Secure                   | [Jetstack Enterprise Registry][] |
| `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent`         | Public  | Tier 1 of Venafi TLS Protect for Kubernetes |                                  |
| `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent` | Private | Tier 2 of Venafi TLS Protect for Kubernetes |                                  |
| `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent`    | Private | Tier 2 of Venafi TLS Protect for Kubernetes |                                  |


## Release Process

> [!NOTE]
> Before starting, let Michael McLoughlin know that a release is about to be created.

The release process is semi-automated.

### Step 1: Incrementing Versions And Git Tag

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
   1. Commit the changes.
1. Create a pull request and wait for it to be approved.
1. Merge the branch.
1. Go to the GitHub Releases page and click "Draft a New Release".
   - Click "Create a new tag" with the version number prefixed with `v` (e.g., `v0.1.49`).
   - Use the title "v0.1.49",
   - Click "Generate Release Notes"
   - Edit the release notes to make them readable to the end-user.
   - Click "Publish" (don't select "Draft")

> [!WARNING]
> 
> Don't worry about the "signing" pipeline job failing. It hasn't be working for a while. It should be removed as we don't need the provenance steps anymore. We are now signing our image during the replication of the OCI images to Harbor using the Venafi keys.

> [!NOTE]
>
> For context, the new tag will trigger the following:
>
> | Image                                                     | Automation                                                                     |
> | --------------------------------------------------------- | ------------------------------------------------------------------------------ |
> | `quay.io/jetstack/preflight`                              | Built by GitHub Actions [release-master](.github/workflows/release-master.yml) |
> | `quay.io/jetstack/venafi-agent`                           | Built by GitHub Actions [release-master](.github/workflows/release-master.yml) |
> | `registry.venafi.cloud/venafi-agent/venafi-agent`         | Mirrored by a GitLab cron job                                                  |
> | `private-registry.venafi.cloud/venafi-agent/venafi-agent` | Mirrored by a GitLab cron job                                                  |
> | `private-registry.venafi.eu/venafi-agent/venafi-agent`    | Mirrored by a GitLab cron job                                                  |
>
> The above GitLab cron job is managed by David Barranco. It mirrors the image
> `quay.io/jetstack/venafi-agent`.

### Step 2: Release the Helm Chart "venafi-kubernetes-agent"

The [venafi-kubernetes-agent](deploy/charts/venafi-kubernetes-agent/README.md) chart
is released manually, as follows:

```sh
export VERSION=0.1.43
helm package deploy/charts/venafi-kubernetes-agent --version "${VERSION}"
docker login -u oauth2accesstoken --password-stdin eu.gcr.io < <(gcloud auth application-default print-access-token)
helm push venafi-kubernetes-agent-${VERSION}.tgz oci://eu.gcr.io/jetstack-secure-enterprise/charts
```

> ℹ️ To test the Helm chart before releasing it, use a [pre-release suffix](https://semver.org/#spec-item-9). E.g.
> `export VERSION=0.1.43-alpha.0`.

The chart will be mirrored to:
 * `registry.venafi.cloud/charts/venafi-kubernetes-agent` (Public)
 * `private-registry.venafi.cloud/charts/venafi-kubernetes-agent` (Private, US)
 * `private-registry.venafi.eu/charts/venafi-kubernetes-agent` (Private, EU)

### Step 3: Release the Helm Chart "jetstack-secure"

This step is performed by Peter Fiddes and Adrian Lai separately from the main
release process.

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
   1. Update the Helm unit test snapshots:
       ```sh
       helm unittest ./deploy/charts/jetstack-agent --update-snapshot
       ```
1. Create a pull request and wait for it to be approved.
1. Merge the branch
1. Push a tag, using the format: `chart-vX.Y.Z`.
   This unique tag format is recognized by the private CI pipeline that builds and publishes the chart.

The chart will be published to
the [Jetstack Enterprise Registry](https://platform.jetstack.io/documentation/installation/agent#1-obtain-oci-registry-credentials)
by a private CI pipeline managed by Venafi.

### Step 4: Document the release

Finally, inform Michael McLoughlin of the new release so he can update the documentation at https://docs.venafi.cloud/.

