## R ShinyApp Operator

This controller doesn't do anything useful.
It's just an example skeleton for writing Metacontroller hooks with Go.

**WARNING**

There's a [known issue](https://github.com/GoogleCloudPlatform/metacontroller/issues/76)
that makes it difficult to produce JSON according to the rules that Metacontroller
requires if you import the official Go structs for Kubernetes APIs.
In particular, some fields will always be emitted, even if you never set them,
which goes against Metacontroller's [apply semantics](https://metacontroller.app/api/apply/).

### Prerequisites

* [Install Metacontroller](https://metacontroller.app/guide/install/)

### Install Thing Controller

```sh
kubectl apply -f thing-controller.yaml
```

### Create a Thing

```sh
kubectl apply -f my-thing.yaml
```

Look at the thing:

```sh
kubectl get thing -o yaml
```

Look at the thing the thing created:

```sh
kubectl get pod thing-1 -a
```

Look at what the thing the thing created said:

```sh
kubectl logs thing-1
```

### Clean up

```sh
kubectl delete -f thing-controller.yaml
```

### Building

You don't need to build to run the example above,
but if you make changes:

```sh
go get -u github.com/golang/dep/cmd/dep
dep ensure
go build -o thing-controller
```

Or just make a new container image:

```sh
docker build . -t <yourname>/thing-controller
```
# MPI Operator

Developed with [MetaController](https://metacontroller.app/) and based on https://github.com/everpeace/kube-openmpi and https://github.com/kubeflow/mpi-operator.

This MPI Kubernetes [Operator](https://coreos.com/operators/) provides a Kubernetes native interface to building MPI clusters and running jobs.

## Deploy

First you must have MetaController:
```shell
make metacontroller
```

Next deploy the Operator:
```shell
make deploy
```

## Test

An MPI cluster relies on a base image that encapsulates the MPI application dependencies and facilitates the MPI communication.  An example of this is the included `mpibase` image, which can be built using: 
```shell
make build_mpibase && make push_mpibase
```
You can use the default images on [Docker Hub](https://hub.docker.com/r/piersharding/) or you must ensure that you configure your own Docker registry details by setting appropriate values for:
```
PULL_SECRET = "gitlab-registry"
GITLAB_USER = you
REGISTRY_PASSWORD = your-registry-password
GITLAB_USER_EMAIL = "you@somewhere.net"
CI_REGISTRY = gitlab.somewhere.com
CI_REPOSITORY = repository/uri
MPIBASE_IMAGE = $(CI_REGISTRY)/$(CI_REPOSITORY)/mpibase:latest
```
set in PrivateRules.mak


Launch the helloworld job:
```shell
make test
```

Once everything starts, the logs are available in the `launcher` pod.

## Scheduling modes

The CRD for MPIJobs has two parameters: `replicas(int)` and `daemons(boolean)`.  Specifying only `replicas` will leave it up to the scheduler where to place the worker pods on the cluster, but if in addition `daemons` is set to `true` (see [mpi-test-demons.yaml](https://github.com/piersharding/metacontroller-mpi-operator/blob/master/mpi-test-daemons.yaml)) then the Pod AntiAffinity rules are applied and the Kubernetes scheduler will force the workers onto individual nodes - if available.
initContainers check availability of the workers, prior to executing the `launcher`, so if any Pods are stuck in `Pending` then they are dropped out of the worker list.