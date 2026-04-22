# Release Process

> [!NOTE]
> Before starting a release let the docs team know that a release is about to be created so that documentation can be prepared in advance.
> This is not necessary for pre-releases.

The release process is semi-automated.

### Step 1: Git Tag and GitHub Release

> [!NOTE]
>
> Upon pushing the tag, a GitHub Action will do the following:
>
> - Build and publish the container image: `quay.io/jetstack/venafi-agent`,
> - Build and publish the Helm chart: `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`,
> - Build and publish the container image: `quay.io/jetstack/disco-agent`,
> - Build and publish the Helm chart: `oci://quay.io/jetstack/charts/disco-agent`,
> - Build and publish the container image: `quay.io/jetstack/discovery-agent`,
> - Build and publish the Helm chart: `oci://quay.io/jetstack/charts/discovery-agent`,
> - Create a draft GitHub release,

1. Run govulncheck; it's the best indicator that a dependency needs to be upgraded.

   ```bash
   make verify-govulncheck
   ```

   Any failures should be treated extremely seriously and patched before release unless you can be absolutely
   confident it's a false positive.

2. Consider upgrading Go dependencies using `go-mod-upgrade`:

   ```bash
   go install github.com/oligot/go-mod-upgrade@latest
   go-mod-upgrade
   make generate
   ```

   Once complete, you'll need to create a PR to merge the changes.

3. Open the [tests GitHub Actions workflow][tests-workflow]
   and verify that it succeeds on the master branch.

4. Create a tag for the new release:

   ```sh
   export VERSION=v1.1.0
   git tag --annotate --message="Release ${VERSION}" "${VERSION}"
   git push origin "${VERSION}"
   ```

   This triggers a [release action](https://github.com/jetstack/jetstack-secure/actions/workflows/release.yml).

5. Wait until the release action finishes.

6. Navigate to the [GitHub Releases](https://github.com/jetstack/jetstack-secure/releases) page and select the draft release to edit.

   1. Click on “Generate release notes” to automatically compile the changelog.
   2. Review and refine the generated notes to ensure they’re clear and useful
      for end users.
   3. Remove any irrelevant entries, such as “update deps,” “update CI,” “update
      docs,” or similar internal changes that do not impact user functionality.

7. Publish the release.

8. Inform the `#venafi-kubernetes-agent` channel on Slack that a new version of the Discovery Agent has been released!
   Consider also messaging the DisCo team at CyberArk (ask in the cert-manager team Slack channel if you don't know who to message)

9. Inform the docs team of the new release so they can update the
   documentation at <https://docs.cyberark.com/>.

[tests-workflow]: https://github.com/jetstack/jetstack-secure/actions/workflows/tests.yaml?query=branch%3Amaster

## Release Artifact Information

For context, the new tag will create the following images:

| Image                                                                | Automation                                                                                   |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `quay.io/jetstack/venafi-agent`                                      | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `quay.io/jetstack/disco-agent`                                       | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `quay.io/jetstack/discovery-agent`                                   | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `registry.venafi.cloud/venafi-agent/venafi-agent`                    | Automatically mirrored by Harbor Replication rule                                            |
| `private-registry.venafi.cloud/venafi-agent/venafi-agent`            | Automatically mirrored by Harbor Replication rule                                            |
| `private-registry.venafi.eu/venafi-agent/venafi-agent`               | Automatically mirrored by Harbor Replication rule                                            |
| `registry.ngts.paloaltonetworks.com/disco-agent/disco-agent`         | Automatically mirrored by Harbor Replication rule                                            |
| `registry.ngts.paloaltonetworks.com/discovery-agent/discovery-agent` | Automatically mirrored by Harbor Replication rule                                            |

and the following OCI Helm charts:

| Helm Chart                                                           | Automation                                                                                   |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`              | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `oci://quay.io/jetstack/charts/disco-agent`                          | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `oci://quay.io/jetstack/charts/discovery-agent`                      | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent`         | Automatically mirrored by Harbor Replication rule                                            |
| `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent` | Automatically mirrored by Harbor Replication rule                                            |
| `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent`    | Automatically mirrored by Harbor Replication rule                                            |
| `oci://registry.ngts.paloaltonetworks.com/charts/disco-agent`        | Automatically mirrored by Harbor Replication rule                                            |
| `oci://registry.ngts.paloaltonetworks.com/charts/discovery-agent`    | Automatically mirrored by Harbor Replication rule                                            |

### Replication Flows

TODO: These flows are helpful illustrations but describe a process whose source of truth is defined elsewhere. Instead, we should document the replication process where it's defined, in enterprise-builds.

Replication flow for the venafi-kubernetes-agent Helm chart:

```text
v1.1.0 (Git tag in the jetstack-secure repo)
 └── oci://quay.io/jetstack/charts/venafi-kubernetes-agent --version 1.1.0 (GitHub Actions in the jetstack-secure repo)
    └── oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent (Enterprise Builds's GitHub Actions)
        ├── oci://registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
        └── oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
        └── oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
```

Replication flow for the venafi-kubernetes-agent container image:

```text
v1.1.0 (Git tag in the jetstack-secure repo)
 └── quay.io/jetstack/venafi-agent:v1.1.0 (GitHub Actions in the jetstack-secure repo)
     └── eu.gcr.io/jetstack-secure-enterprise/venafi-agent:v1.1.0 (Enterprise Builds's GitHub Actions)
         ├── registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
         ├── private-registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
         └── private-registry.venafi.eu/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
```

[public-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/public-registry/module/subsystems/tlspk/replication.tf
[private-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/private-registry/module/subsystems/tlspk/replication.tf
[release_enterprise_builds.yaml]: https://github.com/jetstack/enterprise-builds/actions/workflows/release_enterprise_builds.yaml

## Step 2: Testing

When a release is complete, consider installing it into a cluster and testing it. TODO: provide guidance on doing those tests.
