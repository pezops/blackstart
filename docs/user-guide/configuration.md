<span class="mkdocs-hidden">&larr; [User Guide](README.md)</span>

# Configuration

This page documents runtime configuration and chart values used to run Blackstart in Kubernetes.

## Runtime Flags and Environment Variables

Blackstart supports command-line flags and equivalent environment variables.

| Flag                             | Env Var                                   | Description                                                                               |
| -------------------------------- | ----------------------------------------- | ----------------------------------------------------------------------------------------- |
| `--version`                      | n/a                                       | Print version and exit.                                                                   |
| `--log-output`                   | `BLACKSTART_LOG_OUTPUT`                   | File path for log output. Empty means stdout.                                             |
| `--log-format`                   | `BLACKSTART_LOG_FORMAT`                   | Log format: `text` or `json`.                                                             |
| `--log-level`                    | `BLACKSTART_LOG_LEVEL`                    | Log level, for example `info` or `debug`.                                                 |
| `--log-level-key`                | `BLACKSTART_LOG_LEVEL_KEY`                | JSON key name for log level (for example `level` or `severity`).                          |
| `--log-message-key`              | `BLACKSTART_LOG_MESSAGE_KEY`              | JSON key name for log message (for example `msg`, `message`, or `event`).                 |
| `-f, --workflow-file`            | `BLACKSTART_WORKFLOW_FILE`                | Run a single workflow from a local file instead of Kubernetes.                            |
| `-n, --k8s-namespace`            | `BLACKSTART_K8S_NAMESPACE`                | Comma-separated namespaces to read `Workflow` resources from. Empty means all namespaces. |
| `--runtime-mode`                 | `BLACKSTART_RUNTIME_MODE`                 | Runtime mode for Kubernetes workflows: `controller` (default) or `once`.                  |
| `--max-parallel-reconciliations` | `BLACKSTART_MAX_PARALLEL_RECONCILIATIONS` | Max workflows reconciled at once in controller mode.                                      |
| `--controller-resync-interval`   | `BLACKSTART_CONTROLLER_RESYNC_INTERVAL`   | How often controller mode refreshes workflow resources.                                   |
| `--queue-wait-warning-threshold` | `BLACKSTART_QUEUE_WAIT_WARNING_THRESHOLD` | Warn when queued workflows wait longer than this threshold.                               |

## Namespace Behavior

- Empty `BLACKSTART_K8S_NAMESPACE`: query all namespaces.
- One namespace: query only that namespace.
- Comma-separated list: query each namespace and run workflows found in any of them.

## Helm Values

The chart supports these primary values:

| Key                                                                 | Default                            | Purpose                                                                                                         |
| ------------------------------------------------------------------- | ---------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| <code>serviceAccount.<wbr>create</code>                             | `true`                             | Create a dedicated service account for the workload.                                                            |
| <code>serviceAccount.<wbr>name</code>                               | `blackstart`                       | Service account name used by controller and CronJob modes.                                                      |
| <code>serviceAccount.<wbr>annotations</code>                        | `{}`                               | Optional annotations applied to the service account (for example GKE Workload Identity).                        |
| <code>serviceAccount.<wbr>gcpWorkloadIdentity.<wbr>enabled</code>   | `false`                            | Enable GKE Workload Identity linking to a Google Cloud IAM service account.                                     |
| <code>serviceAccount.<wbr>gcpWorkloadIdentity.<wbr>username</code>  | `""`                               | Username portion of the Google service account email (before `@`).                                              |
| <code>serviceAccount.<wbr>gcpWorkloadIdentity.<wbr>projectID</code> | `""`                               | Project ID of the Google service account email (before `.iam.gserviceaccount.com`).                             |
| <code>serviceAccount.<wbr>awsIRSA.<wbr>enabled</code>               | `false`                            | Enable Amazon EKS IAM roles for service accounts (IRSA).                                                        |
| <code>serviceAccount.<wbr>awsIRSA.<wbr>roleARN</code>               | `""`                               | AWS Identity and Access Management (IAM) role ARN to assign.                                                    |
| <code>serviceAccount.<wbr>awsIRSA.<wbr>stsRegionalEndpoints</code>  | `false`                            | Use regional AWS STS endpoints.                                                                                 |
| <code>image.<wbr>registry</code>                                    | `ghcr.io`                          | Container image registry host.                                                                                  |
| <code>image.<wbr>repository</code>                                  | `pezops/blackstart`                | Container image repository path.                                                                                |
| <code>image.<wbr>tag</code>                                         | Chart `appVersion`                 | Container image tag override. Empty uses chart `appVersion`.                                                    |
| <code>image.<wbr>pullPolicy</code>                                  | `IfNotPresent`                     | Kubernetes image pull policy.                                                                                   |
| <code>controller.<wbr>enabled</code>                                | `true`                             | Enable or disable Deployment controller mode.                                                                   |
| <code>controller.<wbr>maxParallelReconciliations</code>             | `4`                                | Maximum parallel workflow reconciliations in controller mode.                                                   |
| <code>controller.<wbr>resyncInterval</code>                         | `15s`                              | Periodic full resync interval used alongside workflow watches in controller mode.                               |
| <code>controller.<wbr>queueWaitWarningThreshold</code>              | `30s`                              | Queue wait time that triggers backlog warnings in controller mode.                                              |
| <code>cronJob.<wbr>enabled</code>                                   | `false`                            | Enable or disable CronJob creation.                                                                             |
| <code>cronJob.<wbr>schedule</code>                                  | `*/3 * * * *`                      | Cron schedule for periodic execution.                                                                           |
| <code>cronJob.<wbr>concurrencyPolicy</code>                         | `Forbid`                           | Concurrency policy for overlapping runs.                                                                        |
| <code>cronJob.<wbr>startingDeadlineSeconds</code>                   | `60`                               | Deadline for starting missed jobs.                                                                              |
| <code>cronJob.<wbr>successfulJobsHistoryLimit</code>                | `3`                                | Retained successful job history.                                                                                |
| <code>cronJob.<wbr>failedJobsHistoryLimit</code>                    | `1`                                | Retained failed job history.                                                                                    |
| `watchAllNamespaces`                                                | `true`                             | Controls cluster-scoped vs namespaced RBAC and namespace-scoped runtime selection (`BLACKSTART_K8S_NAMESPACE`). |
| <code>rbac.<wbr>create</code>                                       | `true`                             | Create RBAC resources for Blackstart.                                                                           |
| <code>rbac.<wbr>rules</code>                                        | Chart defaults (see `values.yaml`) | RBAC rules applied to Role/ClusterRole resources.                                                               |

## CRD Installation

When installing with Helm, the chart installs the `Workflow` CRD automatically.

If you install or update resources manually, install or upgrade the `Workflow` CRD before applying
workflow resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/pezops/blackstart/<release-tag>/config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml
```

Replace `<release-tag>` with the release tag you are deploying.
