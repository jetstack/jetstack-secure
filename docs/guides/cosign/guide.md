# Cosign

> Note: the ['keyless' signing feature](https://github.com/sigstore/cosign/blob/main/KEYLESS.md)
> of `cosign` used here is currently classified as 'experimental'

The jetstack-secure agent container image is signed using
[`cosign`](https://github.com/sigstore/cosign).

An attestation is attached which satisfies the requirements of
[SLSA 1](https://slsa.dev/spec/v0.1/requirements) and a
[CycloneDX Software Bill of Materials](https://cyclonedx.org/) is also provided
that details the dependencies of the image.

This document outlines how to verify the signature, attestation and download
the SBOM with the `cosign` CLI.

## Signature

To verify the container image signature:

1. Ensure `cosign` is installed
2. Configure the signature repository and enable experimental features:

```
export COSIGN_REPOSITORY=ghcr.io/jetstack/jetstack-secure/cosign
export COSIGN_EXPERIMENTAL=1
```

3. Verify the image

```
cosign verify --cert-oidc-issuer https://token.actions.githubusercontent.com quay.io/jetstack/preflight:latest
```

If the container was properly signed then the command should exit successfully.

The `Subject` in the output should be
`https://github.com/jetstack/jetstack-secure/.github/workflows/release-master.yaml@<ref>`,
where `<ref>` is either the `master` branch or a release tag, i.e:

- `refs/branch/master`
- `refs/tags/v0.1.35`

## SLSA Provenance Attestation

To verify and view the SLSA provenance attestation:

1. Ensure `cosign` is installed
2. Configure the signature repository and enable experimental features:

```
export COSIGN_REPOSITORY=ghcr.io/jetstack/jetstack-secure/cosign
export COSIGN_EXPERIMENTAL=1
```

3. Verify and decode the attestation payload:

```
cosign verify-attestation --cert-oidc-issuer https://token.actions.githubusercontent.com quay.io/jetstack/preflight:latest | tail -n 1 | jq -r .payload | base64 -d | jq -r .
```

## Software Bill of Materials (SBOM)

To verify and download the SBOM:

1. Ensure `cosign` is installed
2. Configure the signature repository and enable experimental features:

```
export COSIGN_REPOSITORY=ghcr.io/jetstack/jetstack-secure/cosign
export COSIGN_EXPERIMENTAL=1
```

3. Verify the SBOM

```
cosign verify --attachment sbom --cert-oidc-issuer https://token.actions.githubusercontent.com quay.io/jetstack/preflight:latest
```

If the SBOM was properly signed then the command should exit successfully.

The `Subject` in the output should be
`https://github.com/jetstack/jetstack-secure/.github/workflows/release-master.yaml@<ref>`,
where `<ref>` is either the `master` branch or a release tag, i.e:

- `refs/branch/master`
- `refs/tags/v0.1.35`

4. Download the SBOM

```
cosign download sbom quay.io/jetstack/preflight:latest > bom.xml
```
