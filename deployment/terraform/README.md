# Terraform modules

## Google Cloud Storage bucket for preflight results

Preflight can be configured to upload its results to a GCS bucket.

The module [`results-bucket`](./results-bucket) creates:

- a GCS bucket
- a Service Account intended for Preflight to write results into that bucket

### How to use

- Apply the module: `terraform apply ./results-bucket`. It will ask for the bucket name and the project ID.
- [Generate a json key](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) for the service account that has been created.
- Use the previously generated key with preflight. The following snipped illustrates how to configure the bucket with this service account in `preflight.yaml`:

```yaml
...
outputs:
- type: gcs
  format: json
  bucket-name: preflight-results
  credentials-path: /var/run/secrets/preflight/credentials.json
```
