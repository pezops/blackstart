<span class="mkdocs-hidden">&larr; [User Guide](README)</span>

# Deploy

Blackstart is designed to be run periodically, typically as a cron job or a scheduled task in a
cloud environment. The following sections describe some examples of how to install and run
Blackstart.

## Kubernetes

### Helm Chart

To install Blackstart on Kubernetes, you can use the provided Helm chart. First, add the Blackstart
Helm repository:

```bash
helm repo add blackstart pezops.github.io/blackstart
```

Then, install the chart:

```bash
helm install blackstart blackstart/blackstart
```

This will deploy Blackstart in your Kubernetes cluster with default configurations. You can
customize the installation by providing a `values.yaml` file or using command-line options to
override specific settings.

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
              image: ghcr.io/pezops/blackstart:latest
```

This manifest schedules Blackstart to run every hour. Make sure to create a service account with the
necessary permissions for Blackstart to operate, and install the CRDs before creating any workflow
resources.
