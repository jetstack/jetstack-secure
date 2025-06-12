[![tests](https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml/badge.svg?branch=master&event=push)](https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jetstack/jetstack-secure.svg)](https://pkg.go.dev/github.com/jetstack/jetstack-secure)
[![Go Report Card](https://goreportcard.com/badge/github.com/jetstack/jetstack-secure)](https://goreportcard.com/report/github.com/jetstack/jetstack-secure)

"The agent" manages your machine identities across Cloud Native Kubernetes and OpenShift environments and builds a detailed view of the enterprise security posture.

## Installation

Please [review the documentation](https://platform.jetstack.io/documentation/installation/agent)
for the agent before getting started.

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

- Go collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus client_golang.
- Process collector: via the [default registry](https://github.com/prometheus/client_golang/blob/34e02e282dc4a3cb55ca6441b489ec182e654d59/prometheus/registry.go#L60-L63) in Prometheus client_golang.
- Agent metrics:
- `data_readings_upload_size`: Data readings upload size (in bytes) sent by the jscp in-cluster agent.

## Tiers, Images and Helm Charts

The Docker images are:

| Image                                                     | Access  | Tier                                        | Docs                        |
| --------------------------------------------------------- | ------- | ------------------------------------------- | --------------------------- |
| `quay.io/jetstack/preflight`                              | Public  | Tier 1 and 2 of Jetstack Secure             |                             |
| `quay.io/jetstack/venafi-agent`                           | Public  | Not meant for users, used for mirroring     |                             |
| `registry.venafi.cloud/venafi-agent/venafi-agent`         | Public  | Tier 1 of Venafi TLS Protect for Kubernetes |                             |
| `private-registry.venafi.cloud/venafi-agent/venafi-agent` | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][] |
| `private-registry.venafi.eu/venafi-agent/venafi-agent`    | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][] |

[Jetstack Enterprise Registry]: https://platform.jetstack.io/documentation/installation/agent#1-obtain-oci-registry-credentials/
[Venafi Private Registry]: https://docs.venafi.cloud/vaas/k8s-components/th-guide-confg-access-to-tlspk-enterprise-components/

The Helm charts are:

| Helm Chart                                                                  | Access  | Tier                                        | Access Documentation             |
| --------------------------------------------------------------------------- | ------- | ------------------------------------------- | -------------------------------- |
| `oci://eu.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`          | Private | Tier 2 of Jetstack Secure                   | [Jetstack Enterprise Registry][] |
| `oci://us.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`          | Private | Tier 2 of Jetstack Secure                   | [Jetstack Enterprise Registry][] |
| `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`                     | Public  | Not meant for users, used for mirroring     |                                  |
| `oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent` | Private | Not meant for users, used for mirroring     |                                  |
| `oci://us.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent` | Private | Not meant for users, used for mirroring     |                                  |
| `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent`                | Public  | Tier 1 of Venafi TLS Protect for Kubernetes |                                  |
| `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent`        | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][]      |
| `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent`           | Private | Tier 2 of Venafi TLS Protect for Kubernetes | [Venafi Private Registry][]      |

## Release Process

> [!NOTE]
> Before starting, let Michael McLoughlin know that a release is about to be created.

The release process is semi-automated.

### Step 1: Git Tag and GitHub Release

> [!NOTE]
>
> Upon pushing the tag, a GitHub Action will do the following:
> - Build and publish the container image at `quay.io/jetstack/venafi-agent`,
> - Build and publish the Helm chart at `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`,
> - Create a draft GitHub release,
> - Upload the Helm chart tarball to the GitHub release.

1. Open the [tests GitHub Actions workflow][tests-workflow]
   and verify that it succeeds on the master branch.
2. Run govulncheck:
   ```bash
   go install golang.org/x/vuln/cmd/govulncheck@latest
   govulncheck -v ./...
   ```
3. Create a tag for the new release:
   ```sh
   export VERSION=v1.1.0
   git tag --annotate --message="Release ${VERSION}" "${VERSION}"
   git push origin "${VERSION}"
   ```
4. Wait until the GitHub Actions finishes.
5. Navigate to the GitHub Releases page and select the draft release to edit.
   1. Click on “Generate release notes” to automatically compile the changelog.
   2. Review and refine the generated notes to ensure they’re clear and useful
      for end users.
   3. Remove any irrelevant entries, such as “update deps,” “update CI,” “update
      docs,” or similar internal changes that do not impact user functionality.
6. Publish the release.
7. Inform the `#venctl` channel that a new version of Venafi Kubernetes Agent has been
   released. Make sure to share any breaking change that may affect `venctl connect`
   or `venctl generate`.
8. Inform Michael McLoughlin of the new release so he can update the
   documentation at <https://docs.venafi.cloud/>.

[tests-workflow]: https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml?query=branch%3Amaster

> [!NOTE]
>
> For context, the new tag will create the following images:
>
> | Image                                                     | Automation                                                                                                                                                                                              |
> | --------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
> | `quay.io/jetstack/preflight`                              | No longer built. Use `quay.io/jetstack/venafi-agent` instead.                                                                                                                                           |
> | `quay.io/jetstack/venafi-agent`                           | Automatically built by GitHub Actions [release-master](.github/workflows/release-master.yml) on Git tags                                                                                                |
> | `registry.venafi.cloud/venafi-agent/venafi-agent`         | Automatically mirrored by Harbor Replication rule [public-img-and-chart-replication.tf][] that runs every 30 minutes, all image tags containing `X.X.X` are replicated, including e.g. `1.0.0-alpha.0`  |
> | `private-registry.venafi.cloud/venafi-agent/venafi-agent` | Automatically mirrored by Harbor Replication rule [private-img-and-chart-replication.tf][] that runs every 10 minutes, all image tags containing `X.X.X` are replicated, including e.g. `1.0.0-alpha.0` |
> | `private-registry.venafi.eu/venafi-agent/venafi-agent`    | Automatically mirrored by Harbor Replication rule [private-img-and-chart-replication.tf][] that runs every 10 minutes, all image tags containing `X.X.X` are replicated, including e.g. `1.0.0-alpha.0` |
>
> and the following OCI Helm charts:
>
> | Helm Chart                                                                  | Automation                                                                                                                                                                                               |
> | --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
> | `oci://eu.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`          | Manually triggered, GitHub Actions workflow [release_venafi-agent_chart.yaml][]                                                                                                                          |
> | `oci://us.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`          | Manually triggered, GitHub Actions workflow [release_venafi-agent_chart.yaml][]                                                                                                                          |
> | `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`                     | Automatically built by GitHub Actions [release-master](.github/workflows/release-master.yml) on Git tags[]                                                                                               |
> | `oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent` | Automatically built by GitHub Actions [release_enterprise_builds.yaml][]                                                                                                                              |
> | `oci://us.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent` | Automatically built by GitHub Actions [release_enterprise_builds.yaml][]                                                                                                                              |
> | `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent`                | Automatically mirrored by Harbor Replication rule [public-img-and-chart-replication.tf][] that runs every 30 minutes, all image tags containing `X.X.X` are replicated, including e.g. `v1.0.0-alpha.0`  |
> | `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent`        | Automatically mirrored by Harbor Replication rule [private-img-and-chart-replication.tf][] that runs every 10 minutes, all image tags containing `X.X.X` are replicated, including e.g. `v1.0.0-alpha.0` |
> | `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent`           | Automatically mirrored by Harbor Replication rule [private-img-and-chart-replication.tf][] that runs every 10 minutes, all image tags containing `X.X.X` are replicated, including e.g. `v1.0.0-alpha.0` |
>
> Here is replication flow for OCI Helm charts:
>
> ```text
> v1.1.0 (Git tag in the jetstack-secure repo)
>  └── oci://quay.io/jetstack/charts/venafi-kubernetes-agent --version 1.1.0 (GitHub Actions in the jetstack-secure repo)
>     ├── oci://us.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent (Enterprise Builds's GitHub Actions)
>     └── oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent (Enterprise Builds's GitHub Actions)
>         ├── oci://registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
>         └── oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
>         └── oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
> ```
>
> And the replication flow for Docker images:
>
> ```text
> v1.1.0 (Git tag in the jetstack-secure repo)
>  └── quay.io/jetstack/venafi-agent:v1.1.0 (GitHub Actions in the jetstack-secure repo)
>      ├── us.gcr.io/jetstack-secure-enterprise/venafi-agent:v1.1.0 (Enterprise Builds's GitHub Actions)
>      └── eu.gcr.io/jetstack-secure-enterprise/venafi-agent:v1.1.0 (Enterprise Builds's GitHub Actions)
>          ├── registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
>          ├── private-registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
>          └── private-registry.venafi.eu/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
> ```

[public-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/public-registry/module/subsystems/tlspk/replication.tf
[private-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/private-registry/module/subsystems/tlspk/replication.tf
[release_venafi-agent_chart.yaml]: https://github.com/jetstack/enterprise-builds/blob/main/.github/workflows/release_venafi-agent_chart.yaml
[release_enterprise_builds.yaml]: https://github.com/jetstack/enterprise-builds/actions/workflows/release_enterprise_builds.yaml

### Step 2: Test the Helm chart "venafi-kubernetes-agent" with venctl connect

NOTE(mael): TBD

### (Optional) Step 3: Release the Helm Chart "jetstack-secure"

This step is performed by Peter Fiddes and Adrian Lai separately from the main
release process.

The `jetstack-secure` chart is for [Jetstack
Secure](https://platform.jetstack.io/documentation/installation/agent#jetstack-agent-helm-chart-installation).
It is composed of two OCI Helm charts:

- `oci://eu.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`
- `oci://us.gcr.io/jetstack-secure-enterprise/charts/jetstack-agent`

> [!NOTE]
>
> The [jetstack-agent](deploy/charts/jetstack-agent/README.md) chart has a
> different version number to the agent. This is because the first version of
> _this_ chart was given version `0.1.0`, while the app version at the time was
> `0.1.38`. And this allows the chart to be updated and released more frequently
> than the Docker image if necessary.

The process is as follows:

1. Create a branch.
2. Increment version numbers.
   1. Increment the `version` value in [Chart.yaml](deploy/charts/jetstack-agent/Chart.yaml).
      DO NOT use a `v` prefix.
      The `v` prefix [breaks Helm OCI operations](https://github.com/helm/helm/issues/11107).
   2. Increment the `appVersion` value in [Chart.yaml](deploy/charts/jetstack-agent/Chart.yaml).
      Use a `v` prefix, to match the Docker image tag.
   3. Increment the `image.tag` value in [values.yaml](deploy/charts/jetstack-agent/values.yaml).
      Use a `v` prefix, to match the Docker image tag.
   4. Update the Helm unit test snapshots:
      ```sh
      helm unittest ./deploy/charts/jetstack-agent --update-snapshot
      ```
3. Create a pull request and wait for it to be approved.
4. Merge the branch
5. Manually trigger the Helm Chart workflow:
   [release_js-agent_chart.yaml](https://github.com/jetstack/enterprise-builds/actions/workflows/release_js-agent_chart.yaml).
