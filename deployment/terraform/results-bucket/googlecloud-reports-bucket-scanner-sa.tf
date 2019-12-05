# This creates the service account that Preflight scanner can use to push the reports to the bucket.

resource "google_service_account" "preflight-scanner-reports-writer-sa" {
  account_id   = "preflight-scanner-reports"
  display_name = "Service account for Preflight to write the reports in the destination bucket"
}

resource "google_storage_bucket_iam_binding" "preflight-scanner-reports-writer-binding" {
  bucket = var.reports_bucket_name
  role   = "roles/storage.objectCreator"

  members = [
    "serviceAccount:${google_service_account.preflight-scanner-reports-writer-sa.email}"
  ]
}

resource "google_service_account_key" "preflight-scanner-reports-writter-key" {
  service_account_id = google_service_account.preflight-scanner-reports-writer-sa.name
}

resource "local_file" "preflight-scanner-reports-writter-key-file" {
  sensitive_content = base64decode(google_service_account_key.preflight-scanner-reports-writter-key.private_key)
  filename = "${path.module}/../../kubernetes/overlays/scanner/secrets/credentials.json"
}
