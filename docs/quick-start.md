# Getting Started

## Install

The Makefile target `deploy-ako-operator` deploys the manager of the load balancer operator.

```bash
make deploy-ako-operator
```

## Local Development

### Setup

Create a CAPD based testing environment with the script `e2e.sh`. This program deploys a local test environment using
Kind and the Cluster API provider for Docker (CAPD).

```bash
make ytt
hack/e2e.sh -u
```

This will also create a management cluster and a workload cluster locally in Docker
for you.

### Run against the mangement cluster

Use `make install` to install the AKODeploymentConfig CRD to the cluster.

```bash
# Set current kubectl context to the local management cluster
kubectl config use-context kind-tkg-lcp

make install

```

Run as a binary locally outside the cluster.

```bash
# Set the default kubeconfig to point to the kind
# tkg-lcp cluster
kubectl config use-context kind-tkg-lcp
Switched to context "kind-tkg-lcp".

# Build the binary
go build -o bin/manager main.go

# Run in the local management cluster
./bin/manager
```

Alternatively, you can run in the kind management cluster as a Deployment

```bash
# Build docker image
make docker-build

# (optional) You may need to login to the registry firstly using your company
# credetials
docker login harbor-pks.vmware.com

# Push the docker image to the VMware internal registry
make docker-push

# Deploy in the management cluster
make deploy
```

### AKODeploymentConfig

AKODeploymentConfig is a Custom Resource to configure how the load balancer operator should manage the load balancer and
ingress resources.

Deploy a AKODeploymentConfig to install AKO automatically in the
workload cluster.

There is one sample in config/samples/network_v1alpha1_akodeploymentconfig.yaml.
Update it with the values in your dev environment.

Then create it in the management cluster

```bash
kubectl apply -f config/samples/network_v1alpha1_akodeploymentconfig.yaml
```

#### Update Containerd Config.toml

If AKO dev registry is used, you need to update the containerd config.toml in
the workload cluster so it's able to pull AKO docker images from this insecure
registry.

```bash
# Find out the workload cluster node image sha
docker ps -a | awk '/workload-cls-worker/{print $1}'
71c3c505ea3d

# Update containerd config.toml
./hack/update-containerd.sh 71c3c505ea3d
```

### Run controller tests

```bash
make integration-test
```

### Run e2e test in kind

```bash
# Create a management cluster and a workload cluster
make ytt
./hack/e2e.sh -u

# Set aliases for accessing both clusters
alias kk="kubectl --kubeconfig=$PWD/tkg-lcp.kubeconfig"
alias kw="kubectl --kubeconfig=$PWD/workload-cls.kubeconfig"

# Set the default kubeconfig to the management cluster
export KUBECONFIG=$PWD/tkg-lcp.kubeconfig

# Build docker image
make docker-build

# Load the docker image into the management cluster
kind load docker-image --name tkg-lcp harbor-pks.vmware.com/tkgextensions/tkg-networking/tanzu-ako-operator:dev

# Deploy in the management cluster
make deploy

# Make sure pod is up and running
➜ git: ✗ kk get pods -n akoo-system
NAME                                       READY   STATUS    RESTARTS   AGE
akoo-controller-manager-757949b86c-6wwn7   2/2     Running   0          3s

# Checking the operator's log

➜ git: ✗ kk logs akoo-controller-manager-757949b86c-6wwn7 -c manager -n akoo-system | tail -n 10
{"level":"info","ts":1604639438.7660556,"logger":"controllers.Cluster","msg":"cluster doesn't have AVI enabled, skip reconciling","Cluster":"default/workload-cls"}
{"level":"info","ts":1604639438.7642214,"logger":"controller-runtime.controller","msg":"Starting EventSource","controller":"machine","source":"kind source: /, Kind="}
{"level":"info","ts":1604639438.7675326,"logger":"controller-runtime.controller","msg":"Starting Controller","controller":"machine"}
{"level":"info","ts":1604639438.7678108,"logger":"controller-runtime.controller","msg":"Starting workers","controller":"machine","worker count":1}
{"level":"info","ts":1604639438.769301,"logger":"controllers.Machine","msg":"Cluster doesn't have AVI enabled, skip reconciling","Machine":"default/workload-cls-worker-0-85c7655bb4-vq6c9","Cluster":"default/workload-cls"}
{"level":"info","ts":1604639438.7707927,"logger":"controllers.Machine","msg":"Cluster doesn't have AVI enabled, skip reconciling","Machine":"default/workload-cls-controlplane-0-4bsrd","Cluster":"default/workload-cls"}
{"level":"info","ts":1604639438.7641554,"logger":"controller-runtime.controller","msg":"Starting Controller","controller":"akodeploymentconfig"}
{"level":"info","ts":1604639438.7752495,"logger":"controller-runtime.controller","msg":"Starting workers","controller":"akodeploymentconfig","worker count":1}

# Open another terminal to watch on the log
➜ git: ✗ kk logs akoo-controller-manager-757949b86c-6wwn7 -c manager -f -n akoo-system

# Enable AVI in the workload cluster
➜ git: ✗ kk label cluster workload-cls cluster-service.network.tkg.tanzu.vmware.com/avi=""
cluster.cluster.x-k8s.io/workload-cls labeled

# Making sure AKO is deployed into the workload cluster
➜ git: ✗ kw get pods  ako-0
NAME    READY   STATUS    RESTARTS   AGE
ako-0   1/1     Running   0          40s

➜ git: ✗ kw get configmap
NAME             DATA   AGE
avi-k8s-config   23     77s

# Making sure finalizer is added on the cluster
➜  ako-operator git:(update-readme) ✗ kk get cluster workload-cls -o yaml  | head
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
...
  finalizers:
  - cluster.cluster.x-k8s.io
  - ako-operator.network.tkg.tanzu.vmware.com

# Making sure the pre-terminate hook is added to the workload cluster Machines
➜ git: ✗ kk get machine -o yaml | grep terminate
      pre-terminate.delete.hook.machine.cluster.x-k8s.io/avi-cleanup: ako-operator
      pre-terminate.delete.hook.machine.cluster.x-k8s.io/avi-cleanup: ako-operator

# Try to delete the workload cluster. This will be a blocking operation, so hit
Ctrl+C to exit
➜ git:(update-readme) ✗ kk delete cluster workload-cls
cluster.cluster.x-k8s.io "workload-cls" deleted

# You should see something similar in the log
{"level":"info","ts":1604640295.9056501,"logger":"controllers.Cluster","msg":"Handling deleted Cluster","Cluster":"default/workload-cls"}
{"level":"info","ts":1604640296.3605769,"logger":"controllers.Cluster","msg":"Found AKO Configmap","Cluster":"default/workload-cls","deleteConfig":"false"}
{"level":"info","ts":1604640296.3606339,"logger":"controllers.Cluster","msg":"Updating deleteConfig in AKO's ConfigMap","Cluster":"default/workload-cls"}
{"level":"info","ts":1604640296.3698053,"logger":"controllers.Cluster","msg":"AKO finished cleanup, updating Cluster condition","Cluster":"default/workload-cls"}
{"level":"info","ts":1604640296.3698819,"logger":"controllers.Cluster","msg":"Removing finalizer","Cluster":"default/workload-cls","finalizer":"ako-operator.network.tkg.tanzu.vmware.com"}

# Check if the cluster is deleted successfully
➜ git: ✗ kk get cluster
No resources found in default namespace.
```

### Enable pprof in your deployment

[Ref](https://gist.github.com/slok/33dad1d0d0bae07977e6d32bcc010188)

```bash
# Add flag to akoo deployment manager args
- args:
   - --metrics-addr=127.0.0.1:8080
   - --profiler-addr=127.0.0.1:8081
   command:
   - /manager
```

```bash
# Expose the pod's port
kubectl port-forward pods/ako-operator-controller-manager-<id> 8081:8081 -n tkg-system-networking
```

```bash
# Get memory profile
curl -s http://127.0.0.1:8081/debug/pprof/heap > ./heap.out
go tool pprof -http=:8080 ./heap.out
```

```bash
# Get CPU profile
curl -s http://127.0.0.1:8081/debug/pprof/profile > ./cpu.out
go tool pprof -http=:8080 ./cpu.out
```

```bash
# Get CPU trace
curl -s http://127.0.0.1:8081/debug/pprof/trace > ./cpu-trace.out
go tool trace -http=:8080 ./cpu-trace.out
```

To get more data, please read [link](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/)
