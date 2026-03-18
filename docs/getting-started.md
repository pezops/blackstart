# Getting Started

## Kubernetes Using Helm

To get started with Blackstart on Kubernetes using the Helm chart, add the PezOps Helm repository
and install the Blackstart chart:

```shell
helm repo add pezops https://pezops.github.io/charts
helm repo update
helm install blackstart pezops/blackstart
```

Install the `Workflow` CRD before creating any workflow resources:

```shell
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with a release tag such as `v0.1.0`.
