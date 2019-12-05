# This creates a bucket where to store the reports from Preflight.

terraform {
  required_version = "~> 0.12"
}

variable "reports_bucket_name" {
  type = string
  description = "The name of the bucket where to store the reports."
}

variable "reports_bucket_gcp_project_id" {
  type        = string
  description = "The ID of the project where the reports are going to be stored."
}

# https://www.terraform.io/docs/providers/google/index.html
provider "google" {
  version = "2.5.1"
  project = var.reports_bucket_gcp_project_id
}

resource "google_storage_bucket" "report_store" {
  name     = var.reports_bucket_name
  location = "EU"
}
