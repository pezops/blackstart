# Primary build tools and paths
PYTHON=python3
GO=go
VENV=.venv
MKDOCS=mkdocs
PIP_COMPILE=pip-compile
SKAFFOLD=skaffold
REQUIREMENTS_IN=docs/requirements.in
REQUIREMENTS_TXT=docs/requirements.txt
CRD_OUT=config/crd
BUILD_TOOLS_DIR=.build
BUILD_TOOLS_DIR_BIN=$(BUILD_TOOLS_DIR)/bin

# CRD API versions to generate CRDs for
CRD_VERSIONS= \
  v1alpha1
# List of CRD YAMLs expected
CRD_YAMLS = \
  config/crd/v1alpha1/blackstart.pezops.github.io_workflows.yaml

# Controller-gen: https://github.com/kubernetes-sigs/controller-tools
CONTROLLER_GEN_VERSION=v0.19.0
CONTROLLER_GEN=$(BUILD_TOOLS_DIR)/bin/controller-gen

# Prettier: https://prettier.io
PRETTIER_VERSION=3.6.2
PRETTIER_DIR=$(BUILD_TOOLS_DIR)/tools/prettier
PRETTIER=$(BUILD_TOOLS_DIR)/bin/prettier

# GolangCI-Lint: https://golangci-lint.run
GOLANGCI_LINT_VERSION=v2.4.0
GOLANGCI_LINT=$(BUILD_TOOLS_DIR)/bin/golangci-lint

RELEASE ?= 0.0.0-dev

.PHONY: build docs-deps docs-serve crds docs-modules-gen docs-format docs-venv utils blackstart clean

build: utils crds docs lint test

blackstart:
	$(GO) build -o blackstart ./cmd/blackstart

## Utils

utils: controller-gen prettier golangci-lint

controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN):
	@mkdir -p $(BUILD_TOOLS_DIR_BIN)
	GOBIN="$(abspath $(BUILD_TOOLS_DIR_BIN))" go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

prettier: $(PRETTIER)
$(PRETTIER):
	@if ! command -v npm >/dev/null 2>&1; then \
	  echo "Error: npm is not installed. Please install Node.js and npm."; \
	  exit 1; \
	fi
	@if ! command -v node >/dev/null 2>&1; then \
	  echo "Error: node is not installed. Please install Node.js and npm."; \
	  exit 1; \
	fi
	@mkdir -p $(BUILD_TOOLS_DIR_BIN)
	@mkdir -p $(PRETTIER_DIR)
	npm install --prefix $(PRETTIER_DIR) prettier@$(PRETTIER_VERSION)
	ln -sf ../tools/prettier/node_modules/.bin/prettier $(PRETTIER)

golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT):
	@mkdir -p $(BUILD_TOOLS_DIR_BIN)
	GOBIN="$(abspath $(BUILD_TOOLS_DIR_BIN))" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

## Code generation

gen: crds docs

## Code Linting

lint: golangci-lint gen
	$(GOLANGCI_LINT) run -v ./...

## Testing

test: lint
	$(GO) test -v ./...

crds: controller-gen
	@mkdir -p $(CRD_OUT)
	@for api in $(CRD_VERSIONS); do \
  	  echo "Generating CRDs for API version $$api"; \
	  $(CONTROLLER_GEN) crd paths=./api/$$api/... output:crd:dir=$(CRD_OUT)/$$api; \
	  $(CONTROLLER_GEN) object paths=./api/$$api/...; \
	done

docs: docs-modules-gen docs-format docs-requirements
docs-modules-gen:
	$(GO) run internal/module_docs/module_docs.go

docs-format: prettier
	$(PRETTIER) --prose-wrap always --print-width 100 --write docs

docs-requirements: $(REQUIREMENTS_TXT)
$(REQUIREMENTS_TXT): $(REQUIREMENTS_IN) | docs-venv
	$(PYTHON) -m pip install --upgrade pip pip-tools
	$(PIP_COMPILE) --generate-hashes $(REQUIREMENTS_IN)

docs-deps: docs-venv docs-requirements
	$(PYTHON) -m pip install -r $(REQUIREMENTS_TXT)

docs-venv:
	@if [ ! -d $(VENV) ]; then \
	  $(PYTHON) -m venv $(VENV); \
	fi
	. $(VENV)/bin/activate; \
	$(PYTHON) -m pip install --upgrade pip; \

docs-serve: docs-venv
	. $(VENV)/bin/activate; \
	$(MKDOCS) serve

clean:
	@rm -rf $(VENV) $(BUILD_TOOLS_DIR)

dev: build
	$(SKAFFOLD) dev