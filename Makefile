# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

BUILD_DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
GIT_VERSION?=$(shell git describe --tags --dirty 2>/dev/null)
GIT_COMMIT?=$(shell git rev-parse HEAD 2>/dev/null)
GIT_BRANCH?=$(shell git symbolic-ref --short HEAD 2>/dev/null)

BIN_DIR = ${PWD}/bin
ifeq (${GIT_VERSION},)
	GIT_VERSION=${GIT_BRANCH}
endif

IMAGE_REGISTRY?=docker.io
IMAGE_TAG=${GIT_VERSION}
ifeq (${IMAGE_TAG},main)
   IMAGE_TAG = latest
endif
# Image URL to use all building/pushing image targets
IMG ?=  ${IMAGE_REGISTRY}/kubegems/bundle-controller:$(IMAGE_TAG)

GOPACKAGE=$(shell go list -m)
ldflags+=-w -s
ldflags+=-X '${GOPACKAGE}/pkg/version.gitVersion=${GIT_VERSION}'
ldflags+=-X '${GOPACKAGE}/pkg/version.gitCommit=${GIT_COMMIT}'
ldflags+=-X '${GOPACKAGE}/pkg/version.buildDate=${BUILD_DATE}'


##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

generate:crd helm-readme helm-template ## Generate all required files.

crd: ## Generate CRD DeepCopy.
	$(CONTROLLER_GEN) paths="./pkg/apis/..." crd  output:crd:artifacts:config=charts/bundle-controller/crds
	$(CONTROLLER_GEN) paths="./pkg/apis/..." object:headerFile="hack/boilerplate.go.txt"

##@ Build
binaries: ## Build binaries.
	- mkdir -p ${BIN_DIR}
	CGO_ENABLED=0 go build -o ${BIN_DIR}/ -gcflags=all="-N -l" -ldflags="${ldflags}" ${GOPACKAGE}/cmd/...

helm-readme:## Generate helm chart's README.md
	readme-generator -v charts/bundle-controller/values.yaml -r charts/bundle-controller/README.md -m charts/bundle-controller/values.schema.json
	markdownlint --fix charts/bundle-controller/README.md

helm-template:## Template helm chart to install.yaml
	helm template bundle-controller --include-crds --namespace bundle-controller charts/bundle-controller > install.yaml

container: binaries ## Build container image.
ifneq (, $(shell which docker))
	docker build -t ${IMG} .
else
	buildah bud -t ${IMG} .
endif

push: ## Push docker image with the manager.
ifneq (, $(shell which docker))
	docker push ${IMG}
else
	buildah push ${IMG}
endif

CONTROLLER_GEN = ${BIN_DIR}/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	GOBIN=${BIN_DIR} go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.0
