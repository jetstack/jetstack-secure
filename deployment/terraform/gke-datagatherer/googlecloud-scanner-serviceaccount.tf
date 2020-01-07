# This creates a service account in GCP with the permissions that Preflight needs for the GKE data-gatherer.

terraform {
  required_version = "~> 0.12"
}

variable "scanner_gcp_project_id" {
  type        = string
  description = "The ID of the project where the cluster Preflight is going check is."
}

# https://www.terraform.io/docs/providers/google/index.html
provider "google" {
  version = "2.5.1"
  project = var.scanner_gcp_project_id
}

resource "google_service_account" "preflight_scanner_service_account" {
  account_id   = "preflight-scanner"
  display_name = "Service account for getting cluster information with workload identity"
  project      = var.scanner_gcp_project_id
}

resource "google_project_iam_member" "preflight_scanner_cluster_viewer" {
  project = var.scanner_gcp_project_id
  role    = "roles/container.clusterViewer"
  member  = "serviceAccount:${google_service_account.preflight_scanner_service_account.email}"
}

resource "google_project_iam_binding" "preflight_scanner_workload_identity" {
  project = var.scanner_gcp_project_id
  role    = "roles/iam.workloadIdentityUser"
  members = [
    "serviceAccount:${var.scanner_gcp_project_id}.svc.id.goog[preflight-scanner/preflight-scanner]",
  ]
}
