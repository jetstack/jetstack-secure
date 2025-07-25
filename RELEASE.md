# Release Process

> [!NOTE]
> Before starting, let Michael McLoughlin know that a release is about to be created so that documentation can be prepared in advance.

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

## Release Artifact Information

For context, the new tag will create the following images:

| Image                                                     | Automation                                                                                                                                                                                              |
| --------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `quay.io/jetstack/venafi-agent`                           | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `registry.venafi.cloud/venafi-agent/venafi-agent`         | Automatically mirrored by Harbor Replication rule                                            |
| `private-registry.venafi.cloud/venafi-agent/venafi-agent` | Automatically mirrored by Harbor Replication rule                                            |
| `private-registry.venafi.eu/venafi-agent/venafi-agent`    | Automatically mirrored by Harbor Replication rule                                            |

and the following OCI Helm charts:

| Helm Chart                                                           | Automation                                                                                   |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| `oci://quay.io/jetstack/charts/venafi-kubernetes-agent`              | Automatically built by the [release action](.github/workflows/release.yml) on Git tag pushes |
| `oci://registry.venafi.cloud/charts/venafi-kubernetes-agent`         | Automatically mirrored by Harbor Replication rule                                            |
| `oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent` | Automatically mirrored by Harbor Replication rule                                            |
| `oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent`    | Automatically mirrored by Harbor Replication rule                                            |

Here is replication flow for OCI Helm charts:

```text
v1.1.0 (Git tag in the jetstack-secure repo)
 └── oci://quay.io/jetstack/charts/venafi-kubernetes-agent --version 1.1.0 (GitHub Actions in the jetstack-secure repo)
    ├── oci://us.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent (Enterprise Builds's GitHub Actions)
    └── oci://eu.gcr.io/jetstack-secure-enterprise/charts/venafi-kubernetes-agent (Enterprise Builds's GitHub Actions)
        ├── oci://registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
        └── oci://private-registry.venafi.cloud/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
        └── oci://private-registry.venafi.eu/charts/venafi-kubernetes-agent --version 1.1.0 (Harbor Replication)
```

And the replication flow for Docker images:

```text
v1.1.0 (Git tag in the jetstack-secure repo)
 └── quay.io/jetstack/venafi-agent:v1.1.0 (GitHub Actions in the jetstack-secure repo)
     ├── us.gcr.io/jetstack-secure-enterprise/venafi-agent:v1.1.0 (Enterprise Builds's GitHub Actions)
     └── eu.gcr.io/jetstack-secure-enterprise/venafi-agent:v1.1.0 (Enterprise Builds's GitHub Actions)
         ├── registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
         ├── private-registry.venafi.cloud/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
         └── private-registry.venafi.eu/venafi-agent/venafi-agent:v1.1.0 (Harbor Replication)
```

[public-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/public-registry/module/subsystems/tlspk/replication.tf
[private-img-and-chart-replication.tf]: https://gitlab.com/venafi/vaas/delivery/harbor/-/blob/3d114f54092eb44a1deb0edc7c4e8a2d4f855aa2/private-registry/module/subsystems/tlspk/replication.tf
[release_enterprise_builds.yaml]: https://github.com/jetstack/enterprise-builds/actions/workflows/release_enterprise_builds.yaml

### Step 2: Test the Helm chart "venafi-kubernetes-agent" with venctl connect

NOTE(mael): TBD
