# Introduction

Blackstart is a tool that helps automate the boring parts of configuring infrastructure. After
infrastructure is created, there can still be work needed before applications will run correctly.
Assigning database grants, creating per-deployment resources, and configuring service backends are
all examples of tasks where Blackstart can help.

<div class="mkdocs-hidden">
<ul>
  <li><a href="getting-started.md">Getting Started</a></li>
  <li><a href="user-guide/README.md">User Guide</a></li>
  <li><a href="developer-guide/README.md">Developer Guide</a></li>
</ul>
</div>

### Features

<!-- prettier-ignore-start -->

<div class="grid cards" markdown>

-   :material-cloud-check:{ .lg .middle } __Cloud Native__

    ---

    Designed to run in cloud environments and take advantage of cloud-native features.

    [:octicons-arrow-right-24: Deploy](user-guide/deploy.md)

-   :material-stack-overflow:{ .lg .middle } __Declarative__

    ---

    Define the desired state of the system, and Blackstart will make it so.

    [:octicons-arrow-right-24: Workflows](user-guide/workflows.md)

-   :material-shield-sun:{ .lg .middle } __Secure__

    ---

    Avoids less secure patterns, helping to create secure systems by default.

    [:octicons-arrow-right-24: Secure practices](user-guide/secure-practices.md)

-   :material-map-marker-path:{ .lg .middle } __Eventually Consistent__

    ---

    If a step is not ready and the workflow fails, it's ok. The workflow will make progress when it runs again.

    [:octicons-arrow-right-24: Eventual consistency](user-guide/eventual-consistency.md)

</div>

<!-- prettier-ignore-end -->
