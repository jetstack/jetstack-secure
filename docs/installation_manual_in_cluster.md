# Installation Manual: Preflight In-Cluster

This doc explains how to run Preflight inside a GKE cluster, and get periodic reports in a Google Cloud Storage Bucket.

## Prerequisites

- A Google Cloud Platform project where to create the destination Google Cloud Storage Bucket.
- A GKE cluster with Workload Identity enabled to run Preflight (it can be a GKE cluster inside the previous project, for instance).
- `kubectl` 1.14+

We will apply Terraform modules as part of this guide, so Terraform should also be installed. However, if required, these operations can be completed in the GCP console or using `gcloud` instead.

## A Preflight Docker image that includes your Preflight Packages.

For this example, we are going to use the generic image, `quay.io/jetstack/preflight`. It includes the packages from this repository ([preflight-packages](../preflight-packages)).

In case you want to add your own packages, you can create your own Docker image including those:

- Add your packages to the `preflight-packages` directory.
- Execute:
```
export DOCKER_IMAGE='myrepo/myimage'
make build-docker-image
make push-docker-image
```

## Prepare GCP

### Create a bucket where to store the reports and a service account that can write to that bucket

Now we need to create a bucket where to store the reports. We are using `preflight-results` as the name for the bucket here, but you will need to choose a different name.

Execute
```
cd ./deployment/terraform/results-bucket
terraform init
terraform apply
```

It will ask for a name for the bucket and for the ID of the Google Cloud project where the bucket is going to be created.

If it is executed correctly, it will generate the file `deployment/kubernetes/overlay/scanner/secrets/credentials.json` with the key for the writer service account.

### Create another service account that Preflight will use to reach the Google Cloud API (only if we are going to use the `gke` datagatherer)

When Preflight runs inside a GKE cluster, the [`gke` datagatherer](./datagatherers/gke.md) can use [_Workload Identity_ and _Cross-Cluster Identity_](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) to authenticate against the Google Cloud API.

This consists of "linking" a Kubernetes service account (KSA) and a Google Cloud service account (GSA) so when a pod uses that KSA it can reach the Google Cloud API authenticated as the GSA.

The following terraform module creates a GSA with the enough permissions to be used with the `gke` datagatherer and also enables _Workload Identity_ on it.

**Create the service account**

Execute:
```
cd ./deployment/terraform/gke-datagatherer
terraform init
terraform apply
```

It will ask for the Google Cloud project ID where the cluster Preflight is going to check is running.

As a result of applying that, it creates a GSA named `preflight-scanner@[project-id].iam.gserviceaccount.com`.

## Deploy Preflight

**Configure Google Cloud service account**

First, we need to annotate the KSA so it points to the GSA where we have configured _Workload Identity_. Edit `deployment/kubernetes/overlays/scanner/workload-identity.yaml` and make sure you change the annotation `iam.gke.io/gcp-service-account` so it is the GSA we have created in the previous step (`preflight-scanner@[project-id].iam.gserviceaccount.com`).

**Custom Docker Image (optional)**

If you built your own Docker image for Preflight, you need to edit `deployment/kubernetes/overlays/scanner/image.yaml` and change `image` there.

**Preflight configuration**

We also need to customize some things in the configuration file. Edit `deployment/kubernetes/overlays/scanner/config/preflight.yaml` and change:
- `cluster-name`, this is the name of the cluster in the context of Preflight. Will be used in the generated reports.
- `data-gatherers.gke`, make sure `project`, `location` and `cluster` correspond to the GKE cluster you want Preflight to scan.
- `outputs[_].bucket-name`, change it so it points to the bucket you created before.

**Deploy**

Now, you can execute `kubectl apply -k deployment/kubernetes/overlays/scanner` and it will deploy Preflight.

By default it runs every 30 minutes (you can change that by editing `deployment/kubernetes/overlays/scanner/period.yaml`). If you want to trigger an execution now, run `kubectl create job -n=preflight-scanner --from=cronjob/preflight preflight-job`.

## Results

If Preflight runs correctly, some results will appear in the bucket, ordered by cluster name and timestamp: `<cluster-name>/<timestamp>/<package-name>.json`
