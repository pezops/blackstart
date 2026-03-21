<span class="mkdocs-hidden">&larr; [User Guide](README.md)</span>

# Deploy

Blackstart is designed to be run periodically as a Kubernetes controller. The default Helm
installation runs Blackstart in a single-replica Deployment and continuously reconciles `Workflow`
resources on each workflow's configured interval.

## Kubernetes

### Helm Chart

To install Blackstart on Kubernetes, you can use the provided Helm chart. First, add the Blackstart
Helm repository:

```bash
helm repo add blackstart https://pezops.github.io/blackstart/charts
helm repo update
```

Then, install the chart:

```bash
helm install blackstart blackstart/blackstart --version <chart-version> --namespace blackstart --create-namespace
```

Chart packages are published with each GitHub release. This will deploy Blackstart in your
Kubernetes cluster with default configurations. You can customize the installation by providing a
`values.yaml` file or using command-line options to override specific settings.

By default, the chart deploys a controller Deployment (`controller.enabled=true`) and disables the
CronJob mode (`cronJob.enabled=false`).

#### GKE Workload Identity

Review the Google Kubernetes Engine documentation to learn how to authenticate from Kubernetes
workloads to Google Cloud APIs:
[https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).

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
workloads to AWS APIs:
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

If you are installing resources manually, install the CRD before creating any workflow resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with a release tag such as `v0.1.0`.

For runtime flags, environment variables, and Helm values, see [Configuration](./configuration.md).

### Manifest

If you prefer to use a Kubernetes manifest directly, you can create a `CronJob` resource. Here is an
example manifest:

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

The CronJob manifest is a legacy/optional run mode. In most deployments, prefer the chart default
controller mode.
