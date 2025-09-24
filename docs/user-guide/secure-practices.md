<span class="mkdocs-hidden">← [User Guide](README.md)</span>

# Secure Practices

Blackstart is designed to make secure configurations easier to implement. As such, there are a few
important practices that are recommended for runtime, and are enforced for module development.

## Workload Identity

It is recommended that Blackstart run in a way to where it inherits its authorization from the
platform it is running on. Generally, this is referred to as "workload identity". This is a pattern
that is common in Cloud environments and within deployment platforms such as Kubernetes. Most cloud
providers have a way to assign a service account or other identity to a compute workload. The
workload could be a serverless function, a container, or a virtual machine. Additionally, most cloud
providers also have a way to map Kubernetes service account to a cloud identity.

Cloud and platform provider known workload identity solutions:

<!-- prettier-ignore-start -->
- [Google Cloud Service Account](https://cloud.google.com/compute/docs/access/service-accounts)
    - [Google Cloud Run Service Account](https://cloud.google.com/run/docs/securing/service-identity)
    - [Google Kubernetes Engine Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity)
- [AWS IAM Roles](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles.html)
    - [AWS IAM Roles for Lambda](https://docs.aws.amazon.com/lambda/latest/dg/lambda-intro-execution-role.html)
    - [AWS IAM Roles for EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)
    - [AWS IAM Roles for Service Accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [Azure Managed Identity](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview)
- [Kubernetes Service Account Tokens](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/)

<!-- the blank line above is required -->
<!-- prettier-ignore-end -->

Modules in Blackstart can pick up the identity of the platform they are running on and use that
identity to connect to and manage resources within the cloud environment. This is a more secure way
to manage resources than using static credentials or requiring manual resource setup by an engineer.

## Avoid Less Secure Patterns

Modules are given a lot of flexibility in how they are implemented. However, there are a few
patterns that are considered "less secure" and will not be allowed in the Blackstart codebase. This
includes patterns such as configuring static usernames and passwords. Blackstart makes it easy to
avoid these types of patterns, so avoiding them altogether helps remove the accumulation of this
security tech debt.

## Local Network Access

Blackstart is designed to run inside the cloud environment it is managing. This avoids any need to
open up network access to the cloud environment from the public internet either directly or using a
proxy. By running inside the cloud environment, Blackstart can use the local network.

When building resources that are attached to a local network such as a Virtual Private Cloud (VPC),
tools like Terraform must then connect to those resources to further configure them. Resources such
as managed databases or Kubernetes clusters are examples of resources that are likely attached to a
local network. To connect to these resources from outside the VPC, a public IP, VPN, or proxy must
be used. Running Blackstart inside the VPC allows it to connect to these resources without needing
to expose resources to the public internet.

## Stateless

Every execution of a Blackstart workflow fully validates the existing state of the deployed
resources as configured in the workflow. There is no need to store state between runs, and that also
means there is no need to store potentially sensitive information in a state file. This is a
significant security benefit, with only a few minor downsides.
