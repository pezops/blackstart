<span class="mkdocs-hidden">&larr; [User Guide](README.md)</span>

# Configuration

This page documents runtime configuration and chart values used to run Blackstart in Kubernetes.

## Runtime Flags and Environment Variables

Blackstart supports command-line flags and equivalent environment variables.

| Flag                  | Env Var                      | Description                                                                               |
| --------------------- | ---------------------------- | ----------------------------------------------------------------------------------------- |
| `--version`           | n/a                          | Print version and exit.                                                                   |
| `--log-output`        | `BLACKSTART_LOG_OUTPUT`      | File path for log output. Empty means stdout.                                             |
| `--log-format`        | `BLACKSTART_LOG_FORMAT`      | Log format: `text` or `json`.                                                             |
| `--log-level`         | `BLACKSTART_LOG_LEVEL`       | Log level, for example `info` or `debug`.                                                 |
| `--log-level-key`     | `BLACKSTART_LOG_LEVEL_KEY`   | JSON key name for log level (for example `level` or `severity`).                          |
| `--log-message-key`   | `BLACKSTART_LOG_MESSAGE_KEY` | JSON key name for log message (for example `msg`, `message`, or `event`).                 |
| `-f, --workflow-file` | `BLACKSTART_WORKFLOW_FILE`   | Run a single workflow from a local file instead of Kubernetes.                            |
| `-n, --k8s-namespace` | `BLACKSTART_K8S_NAMESPACE`   | Comma-separated namespaces to read `Workflow` resources from. Empty means all namespaces. |

## Namespace Behavior

- Empty `BLACKSTART_K8S_NAMESPACE`: query all namespaces.
- One namespace: query only that namespace.
- Comma-separated list: query each namespace and run workflows found in any of them.

## Helm Values

The chart supports these primary values:

| Key                                                 | Purpose                                                                                                         |
| --------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `serviceAccount.create`                             | Create a dedicated service account for the workload.                                                            |
| `serviceAccount.name`                               | Service account name used by the CronJob.                                                                       |
| `image.registry` / `image.repository` / `image.tag` | Container image settings.                                                                                       |
| `image.pullPolicy`                                  | Kubernetes image pull policy.                                                                                   |
| `cronJob.enabled`                                   | Enable or disable CronJob creation.                                                                             |
| `cronJob.schedule`                                  | Cron schedule for periodic execution.                                                                           |
| `cronJob.concurrencyPolicy`                         | Concurrency policy for overlapping runs.                                                                        |
| `cronJob.startingDeadlineSeconds`                   | Deadline for starting missed jobs.                                                                              |
| `cronJob.successfulJobsHistoryLimit`                | Retained successful job history.                                                                                |
| `cronJob.failedJobsHistoryLimit`                    | Retained failed job history.                                                                                    |
| `watchAllNamespaces`                                | Controls cluster-scoped vs namespaced RBAC and namespace-scoped runtime selection (`BLACKSTART_K8S_NAMESPACE`). |
| `rbac.create`                                       | Create RBAC resources for Blackstart.                                                                           |
| `rbac.rules`                                        | Custom RBAC rules applied to Role/ClusterRole.                                                                  |

## CRD Installation

Install or upgrade the `Workflow` CRD before applying workflow resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with the release tag you are deploying.
