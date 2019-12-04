# Installation Manual: Preflight In-Cluster

This doc explains how to run Preflight inside a GKE cluster, and get periodic reports in a Google Cloud Storage Bucket.

## Prerequisites

- A Google Cloud Platform project where to create the destination Google Cloud Storage Bucket.
- A Kubernetes cluster where to install Preflight (it can be a GKE cluster inside the previous project, for instance).

We will apply Terraform modules in this guide, so having Terraform installed locally is also needed. However, those operations can be done from the GCP console or using `gcloud`, so Terraform is not strictly a requirement.

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

Now we need to [create a bucket](https://cloud.google.com/storage/docs/creating-buckets) where to store the reports. We are using `preflight-results** as the name for the bucket here, but you will need to choose a different name.

Execute
```
terraform init ./deployment/terraform/results-bucket
terraform apply ./deployment/terraform/results-bucket
```

It will ask for a name for the bucket and for the ID of the Google Cloud project where the bucket is going to be created.

If it is executed correctly, it will generate the file `./deployment/kubernetes/overlay/scanner/secrets/credentials.json` with the key for the writer service account.

### Create another service account that Preflight will use to reach the Google Cloud API (only if we are going to use the `gke` datagatherer)

When Preflight runs inside a GKE cluster, the [`gke` datagatherer](./datagatherers/gke.md) can use [_Workload Identity_ and _Cross-Cluster Identity_](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) to authenticate against the Google Cloud API.

This consists of "linking" a Kubernetes service account (KSA) and a Google Cloud service account (GSA) so when a pod uses that KSA it can reach the Google Cloud API authenticated as the GSA.

The following terraform module creates a GSA with the enough permissions to be used with the `gke` datagatherer and also enables _Workload Identity** on it.

**Create the service account**

Execute:
```
terraform init ./deployment/terraform/gke-datagatherer
terraform apply ./deployment/terraform/gke-datagatherer
```

It will ask for the Google Cloud project ID where the cluster Preflight is going to check is running.

As a result of applying that, it creates a GSA named `preflight-scanner@[project-id].iam.gserviceaccount.com`.

## Create an overlay

We provide a Kustomize [base](../deployment/kubernetes/base) and a [sample overlay](../deployment/kubernetes/overlays/sample) to deploy Preflight as a CronJob in your Kubernetes cluster.

Create a new overlay, e.g. `deployment/kubernetes/overlays/myoverlay` (you can copy the example).

Add the service account key (`credentials.json`) to your overlay folder.

Edit `preflight.yaml` as you want. It is important that you configure your bucket name in the output section.

Edit `image.yaml` so it uses the docker image you built before.

## Deploy the CronJob

```
kubectl apply -k ./deployment/kubernetes/overlays/myoverlay
```

## Results

Soon, you will start seeing some results in the bucket.
