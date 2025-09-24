<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Building

## Building with `go build`

To build the `blackstart` command using `go build`, follow these steps:

1. Navigate to the root directory of the project.
2. Run the following command to build the binary:

   ```sh
   go build -o blackstart ./cmd/blackstart
   ```

This will create an executable named `blackstart` in the root directory.

## Building with `ko`

To build a container image using `ko`, follow these steps:

1. Ensure you have `ko` installed. You can install it by following the instructions
   [here](https://github.com/google/ko#installation).
2. Set the `KO_DOCKER_REPO` environment variable to `ko.local` to use a local container name:

   ```sh
   export KO_DOCKER_REPO=ko.local
   ```

3. Run the following command to build the container image:

   ```sh
   ko build ./cmd/blackstart
   ```

This will build the `blackstart` command and create a container image with a local name that is not
pushed to a registry.

## Building with `skaffold`

To build and deploy the `blackstart` command using `skaffold`, follow these steps:

1. Ensure you have `skaffold` installed. You can install it by following the instructions
   [here](httpss://skaffold.dev/docs/install/).
2. Run the following command to build the container image and deploy the Helm chart:

   ```sh
   skaffold dev
   ```

This will build the `blackstart` command, create a container image with a local name, and deploy the
Helm chart to the `rancher-desktop` Kubernetes cluster. The `skaffold dev` command will also watch
for changes to the source code and automatically rebuild and redeploy the application.
