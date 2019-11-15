# Run Preflight In-Cluster

This doc explains how to run Preflight inside a GKE cluster, and get periodic reports in a Google Cloud Storage Bucket.

## Build and push a Preflight image with your packages

This is as easy as running:

```
export IMAGE_NAME=<my image>
make build-docker-image
make push-docker-image
```

It assumes your packages are inside `./preflight-packages`.

## Prepare GCP

### Create a GCP Service Account for Preflight to write results

For this example we will name this account `preflight-reports-writer`. We will assume our GCP project is named `myproject`.

This is how you [create a service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts) using `gcloud`.

```
gcloud iam service-accounts create preflight-reports-writer \
    --description "Writes preflight reports" \
    --display-name "Preflight Reports Writer"
```

Now, we need [a file with a key for the service account](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#iam-service-account-keys-create-gcloud):

```
gcloud iam service-accounts keys create ~/credentials.json \
  --iam-account preflight-report-writer@my-project.iam.gserviceaccount.com
```

### Create a bucket where to store the reports

Now we need to [create a bucket](https://cloud.google.com/storage/docs/creating-buckets) where to store the reports. Let's use `preflight-results` as the name for the bucket.

```
gsutil mb gs://preflight-results/
```

### Allow Service Account to write in the buckets

We need to [set write permissions for the Service Account in the bucket](https://cloud.google.com/storage/docs/gsutil/commands/acl):

```
gsutil acl ch -u  preflight-report-writer@my-project.iam.gserviceaccount.com:W gs://preflight-results
```

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
