# Blackstart

[![Lint and Test](https://github.com/pezops/blackstart/actions/workflows/lint_test.yaml/badge.svg)](https://github.com/pezops/blackstart/actions/workflows/lint_test.yaml)

Blackstart helps automate the boring parts of bootstrapping and configuring infrastructure. It helps
teams achieve — and keep — a secure, desired state without worrying about sensitive state files or
manual toil. To achieve this, Blackstart uses a partially ordered set of operations to produce and
run a workflow for bootstrapping and configuring cloud infrastructure and application deployments
after the initial compute, network, and data infrastructure is deployed. It is designed to be
idempotent and does not store a persistent state, avoiding the concern of storing sensitive data in
state files. It can be run on a periodic basis to ensure that the system is kept in the desired
state.

Blackstart operates on eventual consistency — it is possible for a workflow to not reach completion
because it is waiting on an external operation to complete such as a database table creation. When a
workflow runs, Blackstart builds a directed acyclic graph of operations, processing them in a
topological order. This ensures that operations are run only after their dependencies have been met.
If an operation fails, Blackstart logs the information and tries again during the next run. Failure
is expected during the initial setup and the expectation is that the system will eventually converge
to the desired state as other dependencies and applications that are being deployed do their own
initial configuration such as configuring database schemas.

## Components

### Workflow

A workflow is a partially ordered set of operations that are related and depend on each other. Each
workflow is made up of multiple operations and may make use of many different types of modules.

### Operation

An operation is a discrete step in the workflow. Each provides a configuration for a module to
implement and execute as part of the overall workflow. Each operation provides the inputs and the
dependencies it requires.

### Module

Modules provide the core functionality configured by users in their workflows. A module implements
the interface that allows for extensibility within Blackstart. The core set of logic in Blackstart
is separate from module operations. This allows a concrete module implementation to only worry about
the basic set of operations needed, without worrying about how the module is used.

Each module interfaces with Blackstart by implementing the following:

1. **Info**: A method that returns metadata about the module including inputs, outputs, and a
   description.
2. **Validate**: A method that validates the inputs to the module. This method is called before the
   Check and Set methods.
3. **Check**: A method that checks the current state of the system to determine if the operation
   needs to be run. It also must return correct output values if the module has outputs.
4. **Set**: A method that sets the desired state of the system. This method is only called if the
   check method returns false.

## Logic

### Topological Sorting

The dependencies between operations in a workflow must create a directed acyclic graph (DAG). This
DAG is currently used to topologically sort into a linear set of operations which may be executed in
an order where an operation is not run until all of its dependencies have completed. Because of
this, there is no depth-first execution of operations with parallel execution of independent
branches within the DAG of operations. Additionally, the topological sorting, while guaranteeing
dependencies are executed first, does not guarantee a deterministic ordering of operations to be
executed.

### Basic Workflow

![Basic Workflow](docs/images/workflow-light.svg#gh-light-mode-only)
![Basic Workflow](docs/images/workflow-dark.svg#gh-dark-mode-only)

### Example - Bootstrapping a Database

The following is an example of a workflow that bootstraps a database. The challenge is that tools to
create infrastructure such as Terraform will need to connect to the database after creation to
assign table-level grants. This is a good implementation of declarative security as code, but with
cloud databases it generally requires the connection to the database via a public IP, and it has a
race condition on the table creation that will cause the infrastructure creation to partially fail.

#### Architecture

For infrastructure creation, the DevOps process or CICD pipeline runs in a separate network such as
a management network or a cloud-based CICD service.

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

## Secure by Default

Blackstart is designed to be secure by default and help automate the setup of secure systems. That
said, there is no intent to support insecure configurations or resources. For example, for any
database type that supports workload identity or other dynamic authentication, Blackstart does not
and will not support creating a database user with a static password. Instead, for this example,
Blackstart only supports workload identity or other secure methods of authentication.

## Additional Concepts

### Dependencies

Blackstart uses explicit dependencies between operations to ensure correct execution order. Each
operation in a workflow must be assigned a unique identifier. Operations may define an input as
being from a dependency by referencing the identifier and output of another operation in the
workflow. Inputs that depend on other operations impact the generation of the graph for operation
execution order. An operation will only run after all of its dependencies have successfully
completed.

An operation may also depend on another operation without using its outputs as inputs. This is done
by specifying the identifier of the other operation in the dependencies list. This is useful when an
operation must run after another operation, but does not need any outputs from the dependency.

### Stateless and Idempotent

Blackstart and its modules are stateless and perform idempotent operations. However, because it is
stateless, there are a couple concepts to understand:

1. Idempotency requires authoritative control. Each module must specify what it manages
   authoritatively. For example, a database role may be given 4 permissions on a table. The module
   to do this may exercise authority over the role permissions on that table. If the role already
   exists, that would mean existing permissions would be removed and replaced with what is
   configured in the operation for the module.
2. Deleting an operation may orphan resources or settings. Blackstart provides a `doesNotExist` flag
   to enable deletion of previously configured resources. For example, we can assume a database user
   was created using a module. At a later time, the operation to create that user is removed from
   the configuration, resulting in an orphaned user. To fix this, replace the create user operation
   with an operation to delete the user. The idempotent state of a deleted user is simply a user
   that does not exist.
