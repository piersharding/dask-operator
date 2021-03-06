KUBE_NAMESPACE = dask-operator-system
KUBE_REPORT_NAMESPACE ?= default
WEBHOOK_SERVICE_NAME = dask-operator-webhook-service
CERT_DIR = /tmp/k8s-webhook-server/serving-certs
TEMP_DIRECTORY := $(shell mktemp -d)
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

# Image URL to use all building/pushing image targets
IMG ?= piersharding/dask-operator-controller:latest

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Controller runtime arguments
CONTROLLER_ARGS ?=
TEST_USE_EXISTING_CLUSTER ?= false

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# gitlab-runner configuration
GITLAB_JOB ?= vet ## gitlab-runner Job step to test
RDEBUG ?= ""
TIMEOUT = 86400
EXECUTOR ?= shell
CI_ENVIRONMENT_SLUG ?= development
CI_PIPELINE_ID ?= pipeline$(shell tr -c -d '0123456789abcdefghijklmnopqrstuvwxyz' </dev/urandom | dd bs=8 count=1 2>/dev/null;echo)
CI_JOB_ID ?= job$(shell tr -c -d '0123456789abcdefghijklmnopqrstuvwxyz' </dev/urandom | dd bs=4 count=1 2>/dev/null;echo)
GITLAB_USER ?= ""
CI_BUILD_TOKEN ?= ""
REGISTRY_TOKEN ?= ""
DOCKER_HOST ?= unix:///var/run/docker.sock
DOCKER_VOLUMES ?= /var/run/docker.sock:/var/run/docker.sock
CI_APPLICATION_TAG ?= $(shell git rev-parse --verify --short=8 HEAD)

# PVC name and directory for reports
REPORT_VOLUME ?= daskjob-report-pvc-app1-simple
REPORTS_DIR ?= /reports

-include PrivateRules.mak

.PHONY: k8s show lint deploy delete logs describe namespace test clean run install reports help
.DEFAULT_GOAL := help

all: manifests manager  ## run all

# Run tests
test: generate fmt vet manifests ## run tests
	# go test ./... -coverprofile cover.out
	# go test ./controllers/... -coverprofile cover.out
	rm -rf cover.* cover
	mkdir -p cover
	LOG_LEVEL=DEBUG TEST_USE_EXISTING_CLUSTER=$(TEST_USE_EXISTING_CLUSTER) go test ./api/... ./types/... ./utils/... ./models/... ./controllers/... -coverprofile cover.out.tmp -v
	cat cover.out.tmp | grep -v "zz_generated.deepcopy.go" > cover.out
	go tool cover -html=cover.out -o cover/cover.html
	cp cover.out.tmp cover/cover.out
	go tool cover -func=cover.out | grep 'total:' | awk '{print $$3}'  | tr -d '%' > cover/total.txt
	rm -f cover.out cover.out.tmp

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
	kustomize build config/crd | kubectl create -f - || \
	(kustomize build config/crd | kubectl delete -f -; kustomize build config/crd | kubectl create -f -)

# UnInstall CRDs into a cluster
uninstall: ## uninstall CRDs
	kustomize build config/crd | kubectl delete -f -

namespace: ## create the kubernetes namespace
	kubectl describe namespace $(KUBE_NAMESPACE) || kubectl create namespace $(KUBE_NAMESPACE)

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests ## deploy all
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withoutadmissioncontrol | kubectl create -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deployac: namespace manifests getcert secret ## deploy all with Admission Control Hook
	cd config/manager && kustomize edit set image controller=${IMG}
	kustomize build config/withadmissioncontrol | kubectl create -f -

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

# Run go vet lint checking against code
vet: ## run go vet lint checking against code
	go vet ./...

cleandir:
	sudo rm -rf $(ROOT_DIR)/builds

rjob: cleandir ## run code standards check using gitlab-runner
	if [ -n "$(RDEBUG)" ]; then DEBUG_LEVEL=debug; else DEBUG_LEVEL=warn; fi && \
	gitlab-runner --log-level $${DEBUG_LEVEL} exec $(EXECUTOR) \
	--docker-privileged \
	 --docker-disable-cache=false \
	--docker-host $(DOCKER_HOST) \
	--docker-volumes  $(DOCKER_VOLUMES) \
	--docker-pull-policy always \
	--timeout $(TIMEOUT) \
    --env "DOCKER_HOST=$(DOCKER_HOST)" \
		--env "GITLAB_USER=$(GITLAB_USER)" \
		--env "REGISTRY_TOKEN=$(REGISTRY_TOKEN)" \
		--env "CI_BUILD_TOKEN=$(CI_BUILD_TOKEN)" \
		--env "TRACE=1" \
		--env "DEBUG=1" \
	$(GITLAB_JOB) || true

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
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
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

reports: ## retrieve report from PVC - use something like 'make reports REPORT_VOLUME=daskjob-report-pvc-daskjob-app1-http-ipynb'
	rm -rf $$(pwd)/$(REPORTS_DIR)
	mkdir -p $$(pwd)/$(REPORTS_DIR)
	kubectl -n $(KUBE_REPORT_NAMESPACE) run --quiet $(REPORT_VOLUME) \
	  --overrides='{"spec": {"containers": [{"name": "rescue","image": "busybox:latest","command": ["/bin/sh"], "args": ["-c", "sleep 3600"],"volumeMounts": [{"mountPath": "/$(REPORTS_DIR)","name": "reports"}]}],"volumes": [{"name":"reports","persistentVolumeClaim":{"claimName": "$(REPORT_VOLUME)"}}]}}' \
		--image=busybox:latest --restart=Never
	kubectl -n $(KUBE_REPORT_NAMESPACE) wait --for=condition=Ready pod/$(REPORT_VOLUME) --timeout=300s
	echo "ls -latr /$(REPORTS_DIR)" | kubectl -n $(KUBE_REPORT_NAMESPACE) exec -i $(REPORT_VOLUME) sh
	kubectl cp $(KUBE_REPORT_NAMESPACE)/$(REPORT_VOLUME):/$(REPORTS_DIR) $$(pwd)/$(REPORTS_DIR)
	kubectl -n $(KUBE_REPORT_NAMESPACE) delete pod $(REPORT_VOLUME) --now=true --wait=true

dasklogs: ## show Dask POD logs
	@for i in `kubectl -n $(KUBE_REPORT_NAMESPACE) get pods -l app.kubernetes.io/name=dask-scheduler -o=name`; \
	do \
		echo "---------------------------------------------------"; \
		echo "Logs for $${i}"; \
		echo kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i}; \
		echo kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "initContainer: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "Main Pod logs for $${i}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.containers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "Container: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "---------------------------------------------------"; \
		echo ""; echo ""; echo ""; \
	done
	@for i in `kubectl -n $(KUBE_REPORT_NAMESPACE) get pods -l app.kubernetes.io/name=dask-worker -o=name`; \
	do \
		echo "---------------------------------------------------"; \
		echo "Logs for $${i}"; \
		echo kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i}; \
		echo kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.initContainers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "initContainer: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "Main Pod logs for $${i}"; \
		echo "---------------------------------------------------"; \
		for j in `kubectl -n $(KUBE_REPORT_NAMESPACE) get $${i} -o jsonpath="{.spec.containers[*].name}"`; do \
			RES=`kubectl -n $(KUBE_REPORT_NAMESPACE) logs $${i} -c $${j} 2>/dev/null`; \
			echo "Container: $${j}"; echo "$${RES}"; \
			echo "---------------------------------------------------";\
		done; \
		echo "---------------------------------------------------"; \
		echo ""; echo ""; echo ""; \
	done

help:  ## show this help.
	@echo "make targets:"
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ": .*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo ""; echo "make vars (+defaults):"
	@grep -E '^[0-9a-zA-Z_-]+ \?=.*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = " \\?\\= "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
