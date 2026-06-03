<span class="mkdocs-hidden">&larr; [User Guide](README.md)</span>

# Deploy

Blackstart can run as a Kubernetes controller or as a scheduled one-shot job. The default Helm
installation runs Blackstart in a single-replica Deployment and continuously reconciles `Workflow`
resources on each workflow's configured interval. For Google Cloud deployments that do not need a
Kubernetes controller, the Terraform module deploys Blackstart as a Cloud Run Job triggered by Cloud
Scheduler.

## Kubernetes

### Helm Chart

To install Blackstart on Kubernetes, use the provided Helm chart. First, add the Blackstart Helm
repository:

```bash
helm repo add blackstart https://pezops.github.io/blackstart/charts
helm repo update
```

Then, install the chart:

```bash
helm install blackstart blackstart/blackstart --version <chart-version> --namespace blackstart --create-namespace
```

Chart packages are published with each GitHub release. This deploys Blackstart in the Kubernetes
cluster with default configuration. Customize the installation with a `values.yaml` file or
command-line overrides.

By default, the chart deploys a controller Deployment (`controller.enabled=true`) and disables the
CronJob mode (`cronJob.enabled=false`).

#### GKE Workload Identity

Review the Google Kubernetes Engine documentation to learn how to authenticate from Kubernetes
workloads to Google Cloud APIs:<br>
[https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)

When using Workload Identity Federation for GKE, assign IAM policy bindings to the Kubernetes
service account used by Blackstart (chart-created or pre-existing). This mode requires no
service-account annotation values in the Helm chart.

When using annotation-based GKE Workload Identity, configure native chart values to render
`iam.gke.io/gcp-service-account` on the service account.

Example `values.yaml`:

```yaml
serviceAccount:
  create: true
  name: blackstart
  gcpWorkloadIdentity:
    enabled: true
    username: <service-account-id>
    projectID: <project-id>
```

#### Amazon EKS Workload access to AWS

Review the Workload access to AWS documentation to learn how to authenticate from Kubernetes
workloads to AWS APIs:<br>
[https://docs.aws.amazon.com/eks/latest/userguide/service-accounts.html](https://docs.aws.amazon.com/eks/latest/userguide/service-accounts.html)

When using EKS Pod Identity, associate the IAM role with the Kubernetes service account used by
Blackstart (chart-created or pre-existing). This mode requires no service-account annotation values
in the Helm chart.

When using annotation-based IAM roles for service accounts (IRSA), configure native chart values to
render `eks.amazonaws.com/role-arn` (and optionally regional STS endpoints) on the service account.

Example `values.yaml`:

```yaml
serviceAccount:
  create: true
  name: blackstart
  awsIRSA:
    enabled: true
    roleARN: arn:aws:iam::<account-id>:role/<role-name>
    stsRegionalEndpoints: true
```

### CRD Installation

When installing with Helm, the chart installs the `Workflow` CRD automatically.

For manual installs, install the CRD before creating any workflow resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with a release tag such as `v0.1.0`.

For runtime flags, environment variables, and Helm values, see [Configuration](./configuration.md).

### Manifest

For direct manifest usage, create a `CronJob` resource. Example:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: blackstart
spec:
  schedule: "0 * * * *" # Runs every hour
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: blackstart
          containers:
            - name: blackstart
              image: ghcr.io/pezops/blackstart:<release-version>
```

This manifest schedules Blackstart to run every hour. Make sure to create a service account with the
necessary permissions for Blackstart to operate, and install the CRDs before creating any workflow
resources.

The CronJob manifest is a legacy/optional run mode. In most deployments, running Blackstart as a
controller is preferred.

## Terraform

### Google Cloud Run Job

Use the `cloud-run-job` submodule from the `pezops/blackstart/google` module to deploy Blackstart as
a Google Cloud Run Job. This deployment pattern creates a Cloud Run Job for each Blackstart run and
a Cloud Scheduler trigger for periodic execution.

Registry documentation:

- [Terraform Registry](https://registry.terraform.io/modules/pezops/blackstart/google/latest/submodules/cloud-run-job)
- [OpenTofu Registry](https://search.opentofu.org/module/pezops/blackstart/google/latest/submodule/cloud-run-job)

This option is a good fit when:

- Native GCP resources are the primary targets.
- Blackstart reconciliation should run on a schedule.
- The workflow config is stored with the Terraform or in Google Cloud Storage.
- The job needs Google Cloud service account permissions and optional direct VPC egress.

Example using a workflow YAML object in Google Cloud Storage:

```hcl
module "blackstart_job" {
  source  = "pezops/blackstart/google//modules/cloud-run-job"
  version = "0.0.0"

  project_id = var.project_id
  region     = var.region
  name       = var.name

  image_registry   = google_artifact_registry_repository.ghcr_remote.registry_uri
  image_repository = "pezops/blackstart"
  image_tag        = "0.1.13"

  schedule = var.schedule

  workflow_source     = "gcs"
  workflow_gcs_bucket = google_storage_bucket.workflow.name
  workflow_gcs_object = google_storage_bucket_object.workflow.name

  vpc_subnetwork = local.subnetwork_ref

  depends_on = [
    google_artifact_registry_repository.ghcr_remote
  ]
}
```

The `cloud-run-job` submodule supports two workflow source modes:

- `workflow_source = "env"` stores workflow YAML in an environment variable and points
  `BLACKSTART_WORKFLOW_FILE` at that variable.
- `workflow_source = "gcs"` points `BLACKSTART_WORKFLOW_FILE` at
  `gs://<workflow_gcs_bucket>/<workflow_gcs_object>`.

For `gcs` mode, grant the Cloud Run runtime service account read access to the workflow object. The
runtime service account also needs whatever permissions the workflow modules require. For example, a
workflow that configures Cloud SQL users needs appropriate Cloud SQL IAM permissions.

The `cloud-run-job` submodule configures the Cloud Scheduler caller permission needed to execute the
Cloud Run Job. If you provide your own scheduler or runtime service accounts, make sure those
identities have the required IAM bindings.

Direct VPC egress is configured with `vpc_subnetwork` and optional network tags. Use this when the
workflow needs private access to resources such as Cloud SQL private IP endpoints.
