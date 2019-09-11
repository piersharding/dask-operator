DOCKERFILE ?= Dockerfile ## Which Dockerfile to use for build
OPERATOR := dask-operator
KUBE_NAMESPACE ?= "default"
KUBECTL_VERSION ?= 1.14.1
HELM_VERSION ?= v2.14.0
HELM_CHART = $(OPERATOR)
HELM_RELEASE ?= test
OPERATOR_NAMESPACE ?= metacontroller
CI_REGISTRY ?= docker.io
CI_REPOSITORY ?= piersharding
IMAGE ?= $(CI_REPOSITORY)/$(OPERATOR)
TAG ?= latest
REPLICAS ?= 2
INGRESS_HOST ?= notebook.dask.local scheduler.dask.local

.PHONY: k8s show lint deploy delete logs describe namespace test clean metalogs localip help
.DEFAULT_GOAL := help

# define overrides for above variables in here
-include PrivateRules.mak

k8s: ## Which kubernetes are we connected to
	@echo "Kubernetes cluster-info:"
	@kubectl cluster-info
	@echo ""
	@echo "kubectl version:"
	@kubectl version
	@echo ""
	@echo "Helm version:"
	@helm version --client
	@echo ""
	@echo "Helm plugins:"
	@helm plugin list

check: ## Lint check Operator
	golint -set_exit_status .

build: clean check ## build Operator
	GO111MODULE=on go build -o ./$(OPERATOR)

image: ## build image
	docker build \
	  -t $(OPERATOR):latest -f $(DOCKERFILE) .

push: image ## push image
	docker tag $(OPERATOR):latest $(IMAGE):$(TAG)
	docker push $(IMAGE):$(TAG)

metacontroller:  ## deploy metacontroller
	kubectl create namespace metacontroller
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller-rbac.yaml
	kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/metacontroller/master/manifests/metacontroller.yaml

# test:  ## test operator
# 	IMAGE=$(IMAGE) \
# 	REPLICAS=$(REPLICAS) \
# 	MYHOST=$(MYHOST) \
# 	 envsubst < mpi-test-replicas.yaml | kubectl apply -f - -n $(KUBE_NAMESPACE)

# test-daemons:
# 	IMAGE=$(IMAGE) \
# 	REPLICAS=$(REPLICAS) \
# 	 envsubst < mpi-test-daemons.yaml | kubectl apply -f - -n $(KUBE_NAMESPACE)

logs: ## operator logs
	kubectl get pods -l \
	app.kubernetes.io/instance=$(HELM_RELEASE),app.kubernetes.io/name=dask-controller-dask-operator \
	-n $(OPERATOR_NAMESPACE)
	kubectl logs -l \
	app.kubernetes.io/instance=$(HELM_RELEASE),app.kubernetes.io/name=dask-controller-dask-operator \
	-n $(OPERATOR_NAMESPACE)

metalogs: ## show metacontroller POD logs
	@for i in `kubectl -n metacontroller get pods -l app.kubernetes.io/name=metacontroller -o=name`; \
	do \
	echo "---------------------------------------------------"; \
	echo "Logs for $${i}"; \
	echo kubectl -n metacontroller logs $${i}; \
	echo kubectl -n metacontroller get $${i} -o jsonpath="{.spec.initContainers[*].name}"; \
	echo "---------------------------------------------------"; \
	for j in `kubectl -n metacontroller get $${i} -o jsonpath="{.spec.initContainers[*].name}"`; do \
	RES=`kubectl -n metacontroller logs $${i} -c $${j} 2>/dev/null`; \
	echo "initContainer: $${j}"; echo "$${RES}"; \
	echo "---------------------------------------------------";\
	done; \
	echo "Main Pod logs for $${i}"; \
	echo "---------------------------------------------------"; \
	for j in `kubectl -n metacontroller get $${i} -o jsonpath="{.spec.containers[*].name}"`; do \
	RES=`kubectl -n metacontroller logs $${i} -c $${j} 2>/dev/null`; \
	echo "Container: $${j}"; echo "$${RES}"; \
	echo "---------------------------------------------------";\
	done; \
	echo "---------------------------------------------------"; \
	echo ""; echo ""; echo ""; \
	done

# test-results:  ## show test results (logs)
# 	kubectl get pods -l job-name=mpioperator-test-mpi-launcher
# 	kubectl get pods -l job-name=mpioperator-test-mpi-launcher | \
# 	grep Completed | cut -f1 -d" " | xargs kubectl logs || true

clean:  ## clean before build
	rm -f ./$(OPERATOR)

test-clean:  ## clean down test
	kubectl delete -f mpi-test-replicas.yaml -n $(KUBE_NAMESPACE) || true
	sleep 1

cleanall: clean test-clean delete  ## Clean all

redeploy: clean deploy  ## redeploy operator

namespace: ## create the kubernetes namespace
	kubectl describe namespace $(KUBE_NAMESPACE) || kubectl create namespace $(KUBE_NAMESPACE)

delete_namespace: ## delete the kubernetes namespace
	@if [ "default" == "$(KUBE_NAMESPACE)" ] || [ "kube-system" == "$(KUBE_NAMESPACE)" ]; then \
	echo "You cannot delete Namespace: $(KUBE_NAMESPACE)"; \
	exit 1; \
	else \
	kubectl describe namespace $(KUBE_NAMESPACE) && kubectl delete namespace $(KUBE_NAMESPACE); \
	fi

deploy: namespace check  ## deploy the helm chart
	@helm template charts/$(HELM_CHART)/ --name $(HELM_RELEASE) \
				 --namespace $(OPERATOR_NAMESPACE) \
         --tiller-namespace $(OPERATOR_NAMESPACE) \
				 --set helmTests=false | kubectl -n $(OPERATOR_NAMESPACE) apply -f -

install: namespace  ## install the helm chart (with Tiller)
	@helm tiller run $(OPERATOR_NAMESPACE) -- helm install charts/$(HELM_CHART)/ --name $(HELM_RELEASE) \
		--wait \
		--namespace $(OPERATOR_NAMESPACE) \
		--tiller-namespace $(OPERATOR_NAMESPACE)

helm_delete: ## delete the helm chart release (with Tiller)
	@helm tiller run $(OPERATOR_NAMESPACE) -- helm delete $(HELM_RELEASE) --purge \
		--tiller-namespace $(OPERATOR_NAMESPACE)

show: ## show the helm chart
	@helm template charts/$(HELM_CHART)/ --name $(HELM_RELEASE) \
				 --namespace $(OPERATOR_NAMESPACE) \
         --tiller-namespace $(OPERATOR_NAMESPACE)

lint: ## lint check the helm chart
	@helm lint charts/$(HELM_CHART)/ \
				 --namespace $(OPERATOR_NAMESPACE) \
         --tiller-namespace $(OPERATOR_NAMESPACE)

delete: ## delete the helm chart release
	@helm template charts/$(HELM_CHART)/ --name $(HELM_RELEASE) \
				 --namespace $(OPERATOR_NAMESPACE) \
         --tiller-namespace $(OPERATOR_NAMESPACE) \
		 --set helmTests=false | kubectl -n $(OPERATOR_NAMESPACE) delete -f -

describe: ## describe Pods executed from Helm chart
	@for i in `kubectl -n $(OPERATOR_NAMESPACE) get pods -l app.kubernetes.io/instance=$(HELM_RELEASE) -o=name`; \
	do echo "---------------------------------------------------"; \
	echo "Describe for $${i}"; \
	echo kubectl -n $(OPERATOR_NAMESPACE) describe $${i}; \
	echo "---------------------------------------------------"; \
	kubectl -n $(OPERATOR_NAMESPACE) describe $${i}; \
	echo "---------------------------------------------------"; \
	echo ""; echo ""; echo ""; \
	done

helm_tests:  ## run Helm chart tests
	helm tiller run $(OPERATOR_NAMESPACE) -- helm test $(HELM_RELEASE) --cleanup

helm_dependencies: ## Utility target to install Helm dependencies
	@which helm ; rc=$$?; \
	if [ $$rc != 0 ]; then \
	curl "https://kubernetes-helm.storage.googleapis.com/helm-$(HELM_VERSION)-linux-amd64.tar.gz" | tar zx; \
	mv linux-amd64/helm /usr/bin/; \
	helm init --client-only; \
	fi
	@helm init --client-only
	@if [ ! -d $$HOME/.helm/plugins/helm-tiller ]; then \
	echo "installing tiller plugin..."; \
	helm plugin install https://github.com/rimusz/helm-tiller; \
	fi
	helm version --client
	@helm tiller stop 2>/dev/null || true

kubectl_dependencies: ## Utility target to install K8s dependencies
	@([ -n "$(KUBE_CONFIG_BASE64)" ] && [ -n "$(KUBECONFIG)" ]) || (echo "unset variables [KUBE_CONFIG_BASE64/KUBECONFIG] - abort!"; exit 1)
	@which kubectl ; rc=$$?; \
	if [[ $$rc != 0 ]]; then \
		curl -L -o /usr/bin/kubectl "https://storage.googleapis.com/kubernetes-release/release/$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl"; \
		chmod +x /usr/bin/kubectl; \
		mkdir -p /etc/deploy; \
		echo $(KUBE_CONFIG_BASE64) | base64 -d > $(KUBECONFIG); \
	fi
	@echo -e "\nkubectl client version:"
	@kubectl version --client
	@echo -e "\nkubectl config view:"
	@kubectl config view
	@echo -e "\nkubectl config get-contexts:"
	@kubectl config get-contexts
	@echo -e "\nkubectl version:"
	@kubectl version

localip:  ## set local Minikube IP in /etc/hosts file for Ingress $(INGRESS_HOST)
	@new_ip=`minikube ip` && \
	existing_ip=`grep $(INGRESS_HOST) /etc/hosts || true` && \
	echo "New IP is: $${new_ip}" && \
	echo "Existing IP: $${existing_ip}" && \
	if [ -z "$${existing_ip}" ]; then echo "$${new_ip} $(INGRESS_HOST)" | sudo tee -a /etc/hosts; \
	else sudo perl -i -ne "s/\d+\.\d+.\d+\.\d+/$${new_ip}/ if /$(INGRESS_HOST)/; print" /etc/hosts; fi && \
	echo "/etc/hosts is now: " `grep $(INGRESS_HOST) /etc/hosts`

help:  ## show this help.
	@echo "make targets:"
	@grep -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ": .*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo ""; echo "make vars (+defaults):"
	@grep -E '^[0-9a-zA-Z_-]+ \?=.*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = " \\?\\= "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
