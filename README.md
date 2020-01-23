## Dask Kubernetes Operator

This operator manages [Dask](https://dask.org/) clusters, consisting of a scheduler, workers, and optionally Jupyter Notebook server.  The Operator has been developed using [kubebuilder](https://book.kubebuilder.io/).

**NOTE**

Everything is driven by `make`.  Type `make` to get a list of available targets.

This has been tested on Ubuntu 19.04 and Kubernetes 1.17.1.

### Install The Dask Operator

```sh
make install
make deploy
# undo with make delete
```

### Install The Dask Operator with Admission Control WebHook

```sh
make certs
make install
make deployac
# undo with make deleteac
```

### Launch a Dask resource

For the full Dask resource interface documentation see:
```
kubectl get crds dasks.analytics.piersharding.com -o yaml \
   dasks.analytics.piersharding.com
```

See [config/samples/analytics_v1_dask_simple.yaml](config/samples/analytics_v1_dask_simple.yaml) for a detailed example.

```sh
cat <<EOF | kubectl apply -f -
apiVersion: piersharding.com/v1
kind: Dask
metadata:
  name: app-1
spec:
  # daemon: true # to force one worker per node - excess replicas will not start
  jupyter: true # add a Jupyter notebook server to the cluster
  replicas: 5 # no. of workers
  # disablepolicies: true # disable NetworkPolicy access control
  image: daskdev/dask:2.9.0
  jupyterIngress: notebook.dask.local # DNS name for Jupyter Notebook
  schedulerIngress: scheduler.dask.local # DNS name for Scheduler endpoint
  monitorIngress: monitor.dask.local # the Bokeh monitor endpoint of the Dask Scheduler
  imagePullPolicy: Always
  # pass any of the following Pod Container constructs
  # which will be added to all Pods in the cluster:
  # env:
  # volumes:
  # volumeMounts:
  # imagePullSecrets:
  # affinity:
  # nodeSelector:
  # tolerations:
  # add any of the above elements to notebook:, scheduler: and worker: 
  # to specialise for each cluster resource type eg:
  # worker:
  #   env: {}
  # will configure worker specific env vars
EOF
```

Look at the Dask resource, and associated resources - use `-o wide` to get extended details:

```sh
kubectl  get pod,svc,deployment,ingress,dasks -o wide                                                                            wattle: Wed Sep 18 13:36:14 2019

NAME                                          READY   STATUS    RESTARTS   AGE   IP            NODE       NOMINATED NODE   READINESS GATES
pod/dask-scheduler-app-1-66b868c94b-mql7z     1/1     Running   0          31s   172.17.0.7    minikube   <none>           <none>
pod/dask-worker-app-1-9c45b5c76-blv7s         1/1     Running   0          31s   172.17.0.12   minikube   <none>           <none>
pod/dask-worker-app-1-9c45b5c76-ppsv2         1/1     Running   0          31s   172.17.0.9    minikube   <none>           <none>
pod/dask-worker-app-1-9c45b5c76-rlvmh         1/1     Running   0          31s   172.17.0.11   minikube   <none>           <none>
pod/jupyter-notebook-app-1-7467cdd47d-ms5vw   1/1     Running   0          31s   172.17.0.10   minikube   <none>           <none>

NAME                             TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)             AGE   SELECTOR
service/dask-scheduler-app-1     ClusterIP   10.108.30.61     <none>        8786/TCP,8787/TCP   31s   app.kubernetes.io/instance=app-1,app.kubernetes.io/name=dask-scheduler
service/jupyter-notebook-app-1   ClusterIP   10.100.155.117   <none>        8888/TCP            31s   app.kubernetes.io/instance=app-1,app.kubernetes.io/name=jupyter-noteboo
k
service/kubernetes               ClusterIP   10.96.0.1        <none>        443/TCP             35d   <none>

NAME                                           READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES                          SELECTOR
deployment.extensions/dask-scheduler-app-1     1/1     1            1           31s   scheduler    daskdev/dask:latest             app.kubernetes.io/instance=app-1,app.kuber
netes.io/name=dask-scheduler
deployment.extensions/dask-worker-app-1        3/3     3            3           31s   worker       daskdev/dask:latest             app.kubernetes.io/instance=app-1,app.kuber
netes.io/name=dask-worker
deployment.extensions/jupyter-notebook-app-1   1/1     1            1           31s   jupyter      jupyter/scipy-notebook:latest   app.kubernetes.io/instance=app-1,app.kuber
netes.io/name=jupyter-notebook

NAME                            HOSTS                                      ADDRESS         PORTS   AGE
ingress.extensions/dask-app-1   notebook.dask.local,scheduler.dask.local   192.168.86.47   80      31s

NAME                          COMPONENTS   SUCCEEDED   AGE   STATE     RESOURCES
dask.piersharding.com/app-1   3            3           31s   Running   Ingress: dask-app-1 IP: 192.168.86.47, Hosts: http://notebook.dask.local/, http://scheduler.dask.local
/ status: {"loadBalancer":{"ingress":[{"ip":"192.168.86.47"}]}} - Service: dask-scheduler-app-1 Type: ClusterIP, IP: 10.108.30.61, Ports: scheduler/8786,bokeh/8787 status: {
"loadBalancer":{}} - Service: jupyter-notebook-app-1 Type: ClusterIP, IP: 10.100.155.117, Ports: jupyter/8888 status: {"loadBalancer":{}}
```

### Simple test

Create the following cells, and run while watching the monitors at http://monitor.dask.local :

Create a jupyter notebook with the Python3 engine:
```Jupyter Notebook
import os
from dask.distributed import Client
client = Client(os.environ['DASK_SCHEDULER'])
client
```

Now create a sample workload and trigger the computation:

```Jupyter Notebook
import dask.array as da
x = da.random.random((10000, 10000), chunks=(1000, 1000))
x
```

```Jupyter Notebook
y = x + x.T
z = y[::2, 5000:].mean(axis=1)
z
```

```Jupyter Notebook
z.compute()
```

### Clean up

```sh
kubectl delete dask app-1
```

### Scheduling Notebooks in the Background

Notebooks can be run against the cluster in the background with or without
saving the report output.

The Notebook can either be passed inline, or via a URL reference - see the following examples:

* Inline Notebook - [config/samples/analytics_v1_daskjob_simple_ipynb.yaml](config/samples/analytics_v1_daskjob_simple_ipynb.yaml)
* Notebook sourced from a URL - [config/samples/analytics_v1_daskjob_url_ipynb.yaml](config/samples/analytics_v1_daskjob_url_ipynb.yaml)

In both cases a DaskJob resource schedules a Job resource and can optionally stash the report results in a PersisentVolumeClaim by setting the value `report: true` - see `make reports REPORT_VOLUME=name-of-PersistentVolumeClaim` which will recover the HTML output of the Notebook to `./reports`.

### Building

You don't need to build to run the operator,
but if you would like to make changes:

```sh
make manager
```

Or to make a new container image:

```sh
make image
```

### Help
```sh
$ make
make targets:
Makefile:all                   run all
Makefile:deleteac              delete deployment with Admission Control Hook
Makefile:delete                delete deployment
Makefile:deployac              deploy all with Admission Control Hook
Makefile:deploy                deploy all
Makefile:describe              describe Pods executed from Helm chart
Makefile:docker-build          Build the docker image
Makefile:docker-push           Push the docker image
Makefile:fmt                   run fmt
Makefile:generate              Generate code
Makefile:help                  show this help.
Makefile:image                 build and push
Makefile:install               install CRDs
Makefile:logs                  show Helm chart POD logs
Makefile:manifests             generate mainfests
Makefile:namespace             create the kubernetes namespace
Makefile:run                   run foreground live
Makefile:showcrds              show CRDs
Makefile:show                  show deploy
Makefile:test                  run tests
Makefile:uninstall             uninstall CRDs
Makefile:vet                   run vet

make vars (+defaults):
Makefile:CONTROLLER_ARGS       
Makefile:CRD_OPTIONS           "crd:trivialVersions=true"
Makefile:IMG                   piersharding/dask-operator-controller:latest
```
