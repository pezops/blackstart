# Blackstart

Starting a system from scratch can require a lot of manual work. Blackstart provides an ordered
workflow for bootstrapping cloud infrastructure and application deployments after the initial
compute and other infrastructure is deployed. It is designed to be idempotent, implementing a "check
then set" approach. This means that it can be run on a periodic basis to ensure that the system is
kept in the desired state.

> [!WARNING]  
> Blackstart is still being developed and does not currently have an initial release. Artifacts
> including helm charts and container images are not yet published.

To avoid race conditions, Blackstart operates on eventual consistency. It builds a directed acyclic
graph of operations, processing them in a topological order. This ensures that operations are run
only after their dependencies have been met. If an operation fails, Blackstart logs the information
and tries again during the next run. Failure is expected during the initial setup and the
expectation is that the system will eventually converge to the desired state.

## Components

### Workflow

A workflow is a set of operations that are related and depend on each other. Each workflow is made
up of multiple operations and may make use of many different types of modules in various orders. As
independent groups of operations, multiple workflows may be run in parallel.

### Operation

An operation is a configuration for a module to implement and is a single step in the overall
workflow. Each operation carries metadata about the module it uses, its provided inputs, and the
dependencies it requires.

### Module

A module implements the interface that allows for extensibility within Blackstart. The core set of
logic in Blackstart is separate from module operations. This allows a concrete module implementation
to only worry about the basic set of operations needed, without worrying about how the module is
used.

Each module interfaces with Blackstart by implementing the following:

1. **Validate**: A method that validates the inputs to the module. This method is called before the
   Check and Set methods.
2. **Check**: A method that checks the current state of the system to determine if the operation
   needs to be run. It also must return correct output values if the module has outputs.
3. **Set**: A method that sets the desired state of the system. This method is only called if the
   check method returns false.

## Logic

### Topological Sorting

The dependencies between operations in a workflow must create a directed acyclic graph (DAG). This
DAG is currently used to topologically sort into a linear set of operations which may be executed in
an order where an operation is not run until all of its dependencies have completed. Because of
this, there is no depth-first execution of operations, or parallel execution of independent branches
within the DAG of operations. Additionally, the topological sorting, while guaranteeing dependencies
are executed first, does not provide a deterministic ordering of operations executed.

### Basic Workflow

![Basic Workflow](docs/images/workflow-light.svg#gh-light-mode-only)
![Basic Workflow](docs/images/workflow-dark.svg#gh-dark-mode-only)

### Example - Bootstrapping a Database

The following is an example of a workflow that bootstraps a database. The challenge is that tools to
create infrastructure such as Terraform will need to connect to the database after creation to
assign table-level grants. This is a good implementation of declarative security as code, but it
requires the connecting to the database via a public IP, and it has a race condition on the table
creation that will cause the infrastructure creation to partially fail.

#### Architecture

For infrastructure creation, the DevOps process or CICD pipeline runs in a separate network such as
a management network or a CICD service.

![Architecture](docs/images/ex1-arch-light.svg#gh-light-mode-only)
![Architecture](docs/images/ex1-arch-dark.svg#gh-dark-mode-only)

#### Terraform Only

When deploying this infrastructure with only Terraform, the apply fails because the application has
not been deployed, and the application manages the database schema. It is possible to converge the
infrastructure and application releases, allowing Terraform to manage application releases. However,
this is not ideal as it creates a tight coupling between infrastructure and application releases,
adding to the complexity in larger systems.

![Terraform Only](docs/images/ex1-tf-only-light.svg#gh-light-mode-only)
![Terraform Only](docs/images/ex1-tf-only-dark.svg#gh-dark-mode-only)

#### Terraform and Blackstart

When deploying this infrastructure with Terraform and Blackstart, the core infrastructure is created
by Terraform, and Blackstart handles creating the database grants from inside the VPC.

![Terraform and Blackstart](docs/images/ex1-tf-blackstart-light.svg#gh-light-mode-only)
![Terraform and Blackstart](docs/images/ex1-tf-blackstart-dark.svg#gh-dark-mode-only)

## Additional Concepts

When Blackstart runs, it processes a set of operations to configure an environment. Each operation
is performed by a module that implements the "check then set" interface. Blackstart and its modules
are stateless and perform idempotent operations. However, because it is stateless, there are a
couple concepts to understand:

1. Idempotency requires authoritative control. Each module must specify what it manages
   authoritatively. For example, a database role may be given 4 permissions on a table. The module
   to do this may exercise authority over the role permissions on that table. If the role already
   exists, that would mean existing permissions would be removed and replaced with what is
   configured in the operation for the module.
2. Deleting an operation may orphan resources or settings. For example, we can assume a database
   user was created using a module. At a later time, the operation to create that user is removed
   from the configuration. The user previously created will be orphaned. To fix this, replace the
   create user operation with an operation to delete the user. The idempotent state of a deleted
   user is simply a user that should not exist.

Individual operations may have dependencies on other operations. Using Blackstart, each operation
requires a unique identifier - other operations use this identifier to specify what they depend on
to run. Using this information, Blackstart builds a directed acyclic graph and processes operations
in a topological order. Relying on the dependency graph, Blackstart operates using an eventually
consistent model. If a dependency is not met, Blackstart will log this information and then try
again during the next run. Failures are expected and are okay. For example, an operation may grant
access to a database, and it may depend on an operation to create the database. If the database
creation operation did not complete successfully because the database instance was missing, the
operation to grant access to the database will not run. Once the database instance is up, and the
database is created, the operation to grant access to the database will run.

## Secure by Default

Blackstart is designed to be secure by default and help automate the setup of secure systems. That
said, there is no intent to support insecure configurations or resources. For example, for any
database type that supports workload identity or other dynamic authentication, Blackstart does and
will not support creating a database user with a static password. Instead, Blackstart only supports
workload identity or other secure methods of authentication.
