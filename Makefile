KUBE_NAMESPACE = dask-operator-system
WEBHOOK_SERVICE_NAME = dask-operator-webhook-service
CERT_DIR = /tmp/k8s-webhook-server/serving-certs
TEMP_DIRECTORY := $(shell mktemp -d)

# Image URL to use all building/pushing image targets
IMG ?= piersharding/dask-operator-controller:latest

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Controller runtime arguments
CONTROLLER_ARGS ?= 

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

-include PrivateRules.mak

.DEFAULT_GOAL := help

all: manifests manager  ## run all

# Run tests
test: generate fmt vet manifests ## run tests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

testcerts:
	rm -rf $(CERT_DIR)
	mkdir -p $(CERT_DIR)
	openssl req -x509 -newkey rsa:2048 -keyout $(CERT_DIR)/tls.key -out $(CERT_DIR)/tls.crt -days 365 -nodes -subj "/CN=$(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc"

certs:
	# from https://kubernetes.github.io/ingress-nginx/deploy/validating-webhook/
	rm -rf $(CERT_DIR)
	mkdir -p $(CERT_DIR)
	@echo "[req]\n" \
	"req_extensions = v3_req\n" \
	"distinguished_name = req_distinguished_name\n" \
	"[req_distinguished_name]\n" \
	"[ v3_req ]\n" \
	"basicConstraints = CA:FALSE\n" \
	"keyUsage = nonRepudiation, digitalSignature, keyEncipherment\n" \
	"extendedKeyUsage = serverAuth\n" \
	"subjectAltName = @alt_names\n" \
	"[alt_names]\n" \
	"DNS.1 = $(WEBHOOK_SERVICE_NAME)\n" \
	"DNS.2 = $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE)\n" \
	"DNS.3 = $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc\n" \
	| sed 's/ //' > $(CERT_DIR)/csr.conf
	cat $(CERT_DIR)/csr.conf
	openssl genrsa -out $(CERT_DIR)/tls.key 2048
	openssl req -new -key $(CERT_DIR)/tls.key \
	-subj "/CN=$(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc" \
	-out $(CERT_DIR)/server.csr \
	-config $(CERT_DIR)/csr.conf
	@echo \
	"apiVersion: certificates.k8s.io/v1beta1\n" \
	"kind: CertificateSigningRequest\n" \
	"metadata:\n" \
	"  name: $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc\n" \
	"spec:\n" \
	"  request: $$(cat $(CERT_DIR)/server.csr | base64 | tr -d '\n')\n" \
	"  usages:\n" \
	"  - digital signature\n" \
	"  - key encipherment\n" \
	"  - server auth\n" \
	| sed 's/^ //' > $(CERT_DIR)/approve.yaml
	ls -latr $(CERT_DIR)
	cat $(CERT_DIR)/approve.yaml
	kubectl delete -f $(CERT_DIR)/approve.yaml || true
	kubectl apply -f $(CERT_DIR)/approve.yaml
	sleep 3
	kubectl certificate approve $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc

getcert:
	while true; do \
	STATUS=$$(kubectl get csr $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc  -o jsonpath='{.status.conditions[].type}'); \
	if [ "$${STATUS}" = "Approved" ]; then break; fi; \
	echo "Status is: $${STATUS} - sleeping"; \
	sleep 10; \
	done
	SERVER_CERT=$$(kubectl get csr $(WEBHOOK_SERVICE_NAME).$(KUBE_NAMESPACE).svc  -o jsonpath='{.status.certificate}') && \
	echo $${SERVER_CERT} | openssl base64 -d -A -out $(CERT_DIR)/tls.crt
	rm -rf config/webhook/secret/tls.*
	cp $(CERT_DIR)/tls.crt $(CERT_DIR)/tls.key config/webhook/secret/

secret: namespace
	kubectl delete secret webhook-server-cert -n $(KUBE_NAMESPACE) || true
	kubectl create secret generic webhook-server-cert \
	--from-file=tls.key=$(CERT_DIR)/tls.key \
	--from-file=tls.crt=$(CERT_DIR)/tls.crt \
	-n $(KUBE_NAMESPACE)

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests ## run foreground live
	go run ./main.go $(CONTROLLER_ARGS)

# Install CRDs into a cluster
showcrds: manifests ## show CRDs
	kustomize build config/crd | more

# Install CRDs into a cluster
show: namespace manifests secret getcert ## show deploy
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/default | more

# Install CRDs into a cluster
install: manifests ## install CRDs
	kustomize build config/crd | kubectl create -f -

# UnInstall CRDs into a cluster
uninstall: ## uninstall CRDs
	kustomize build config/crd | kubectl delete -f -

namespace: ## create the kubernetes namespace
	kubectl describe namespace $(KUBE_NAMESPACE) || kubectl create namespace $(KUBE_NAMESPACE)

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests ## deploy all
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withoutadmissioncontrol | kubectl apply -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deployac: namespace manifests getcert secret ## deploy all with Admission Control Hook
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withadmissioncontrol | kubectl apply -f -

delete: ## delete deployment
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withoutadmissioncontrol | kubectl delete -f -

deleteac: ## delete deployment with Admission Control Hook
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withadmissioncontrol | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen ## generate mainfests
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt: ## run fmt
	go fmt ./...

# Run go vet against code
vet: ## run vet
	go vet ./...

# Generate code
generate: controller-gen ## Generate code
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths="./..."

# Build the docker image
docker-build: test ## Build the docker image
	docker build . -t ${IMG}

# Push the docker image
docker-push: ## Push the docker image
	docker push ${IMG}

image: docker-build docker-push ## build and push

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.2 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif


logs: ## show Helm chart POD logs
	@for i in `kubectl -n $(KUBE_NAMESPACE) get pods -l control-plane=controller-manager -o=name`; \
	do \
		echo "---------------------------------------------------"; \
		echo "Logs for $${i}"; \
		echo kubectl -n $(KUBE_NAMESPACE) logs $${i}; \
		echo kubectl -n $(KUBE_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "initContainer: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "Main Pod logs for $${i}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_NAMESPACE) get $${i} -o jsonpath="{.spec.containers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "Container: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "---------------------------------------------------"; \
		echo ""; echo ""; echo ""; \
	done

describe: ## describe Pods executed from Helm chart
	@for i in `kubectl -n $(KUBE_NAMESPACE) get pods -l control-plane=controller-manager -o=name`; \
	do echo "---------------------------------------------------"; \
	echo "Describe for $${i}"; \
	echo kubectl -n $(KUBE_NAMESPACE) describe $${i}; \
	echo "---------------------------------------------------"; \
	kubectl -n $(KUBE_NAMESPACE) describe $${i}; \
	echo "---------------------------------------------------"; \
	echo ""; echo ""; echo ""; \
	done

help:  ## show this help.
	@echo "make targets:"
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ": .*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo ""; echo "make vars (+defaults):"
	@grep -E '^[0-9a-zA-Z_-]+ \?=.*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = " \\?\\= "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
