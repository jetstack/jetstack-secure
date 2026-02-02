# Encrypted Secrets Example

This example demonstrates how to use the disco agent to gather Kubernetes secrets and encrypt their data fields.

## Overview

When the `ARK_SEND_SECRETS` environment variable is set to `"true"`, the disco agent will:

0. Fetch an encryption key from the configured endpoint (if running in production) or use a local key for testing
1. Discover Kubernetes secrets in your cluster (excluding common system secret types)
2. Encrypt each secret's data fields using RSA envelope encryption with JWE (JSON Web Encryption) format
3. If running in production, send the encrypted secrets to the configured endpoint; otherwise, write them to `output.json` for testing

The encryption uses:

- **Key Algorithm**: RSA-OAEP-256 (for encrypting the content encryption key)
- **Content Encryption**: AES-256-GCM (for encrypting the actual secret data)
- **Format**: JWE Compact Serialization

Metadata (names, namespaces, labels, annotations) remains in plaintext for discovery purposes, while the sensitive secret data is encrypted. Some keys in Secret data fields are also preserved in the `data` section, for backwards compatibility.

## Prerequisites

1. A running Kubernetes cluster with secrets to discover
3. Go installed

## Configuration File

The `config.yaml` file configures:

- The data gatherer to collect Kubernetes secrets
- Field selectors to exclude system secrets (service account tokens, docker configs, etc.)
- The cluster ID and organization ID for grouping data

## Running the Example

Test the agent locally by running this script:

```bash
./test.sh
```

This will:

- Connect to your current Kubernetes context
- Gather all non-system secrets
- Write the raw data to `output.json`
