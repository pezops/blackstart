# Getting Started

## Kubernetes Using Helm

To get started with Blackstart on Kubernetes using the Helm chart, add the PezOps Helm repository
and install the Blackstart chart:

```shell
helm repo add pezops https://pezops.github.io/blackstart/charts
helm repo update
helm install blackstart pezops/blackstart
```

Install the `Workflow` CRD before creating any workflow resources:

```shell
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with a release tag such as `v0.1.0`.

## Quick Example

This example installs the chart, creates a `Workflow` that manages a ConfigMap named
`blackstart-example`, and sets one key (`message=hello-from-blackstart`).

```shell
cat <<'YAML' | kubectl apply -f -
apiVersion: blackstart.pezops.github.io/v1alpha1
kind: Workflow
metadata:
  name: blackstart-configmap-example
spec:
  reconcileInterval: 1m
  operations:
    - id: k8s_client
      module: kubernetes_client
    - id: example_configmap
      module: kubernetes_configmap
      inputs:
        client:
          fromDependency:
            id: k8s_client
            output: client
        namespace: default
        name: blackstart-example
    - id: example_configmap_value
      module: kubernetes_configmap_value
      inputs:
        configmap:
          fromDependency:
            id: example_configmap
            output: configmap
        key: message
        value: hello-from-blackstart
YAML
```

Verify:

```shell
kubectl get configmap blackstart-example -n default -o yaml
```
