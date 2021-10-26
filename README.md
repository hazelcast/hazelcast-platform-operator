# Hazelcast Platform Operator

## Quick Start

### Prerequisites

Kubernetes cluster (with admin rights), and the kubectl command configured.

### Step 1: Create Secret with Hazelcast License Key

```shell
kubectl create secret generic hazelcast-license-key --from-literal=license-key=<YOUR LICENSE KEY>
```

### Step 2: Start Hazelcast Platform Operator

```shell
git clone git@github.com:hazelcast/hazelcast-platform-operator.git
cd hazelcast-platform-operator
make deploy IMG=hazelcast/hazelcast-platform-operator:5-preview-snapshot
```

> Note: If you want to run the operator locally, you can execute `make install run` instead of `make deploy`.

> Note: By default, Hazelcast Operator is installed into the `default` namespace. If you want to change it, you can pass `NAMESPACE` to `make` commands.

### Step 3: Start Hazelcast Enterprise Cluster

Run the following command to start Hazelcast Platform Cluster via applying CR yaml:

```shell
kubectl apply -f config/samples/_v1alpha1_hazelcast.yaml
```
```yaml
apiVersion: hazelcast.com/v1alpha1
kind: Hazelcast
metadata:
  name: hazelcast
spec:
  clusterSize: 3
  repository: 'docker.io/hazelcast/hazelcast-enterprise'
  version: '5.0'
  licenseKeySecret: hazelcast-license-key
```

You can check the operator's logs to see the resource creation logs:
```
$ kubectl logs deployment.apps/hazelcast-platform-controller-manager manager
...
2021-06-16T16:37:09.539+0300    DEBUG   controllers.Hazelcast   Finalizer added into custom resource successfully       {"hazelcast": "default/hazelcast"}
2021-06-16T16:37:09.743+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast", "ClusterRole": "hazelcast", "result": "created"}
2021-06-16T16:37:09.932+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast", "ServiceAccount": "hazelcast", "result": "created"}
2021-06-16T16:37:10.370+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast", "RoleBinding": "hazelcast", "result": "created"}
2021-06-16T16:37:10.992+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast", "Statefulset": "hazelcast", "result": "created"}
2021-06-16T16:37:11.291+0300    DEBUG   controllers.Hazelcast   Statefulset resource version has been changed during create/update process.     {"hazelcast": "default/hazelcast"}
2021-06-16T16:37:11.559+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast", "Statefulset": "hazelcast", "result": "updated"}
```

Check Hazelcast member's log:
```shell
$ kubectl logs hazelcast-0

....
Members {size:3, ver:3} [
        Member [10.131.0.130]:5701 - f447da35-0c04-4199-b99d-cb02376ad175 this
        Member [10.131.0.194]:5701 - c6b34055-3937-4adf-b0ae-d30f44495331
        Member [10.131.1.2]:5701 - 0fa2a21a-d159-4cac-ab08-bdac994d912a
]
```
### Step 4: Start Management Center
Run the following command to start Management Center via applying CR yaml:

```shell
kubectl apply -f config/samples/_v1alpha1_managementcenter.yaml
```
```yaml
apiVersion: hazelcast.com/v1alpha1
kind: ManagementCenter
metadata:
  name: managementcenter
spec:
  repository: 'hazelcast/management-center'
  version: '5.0'
  licenseKeySecret: hazelcast-license-key
  externalConnectivity:
    type: LoadBalancer
  hazelcastClusters:
    - address: hazelcast
      name: dev
  persistence:
    enabled: true
    size: 10Gi
```

You can check Management Center's log:

```shell
$ kubectl logs managementcenter-0

...
2021-08-26 15:21:04,842 [ INFO] [MC-Client-dev.lifecycle-1] [c.h.w.s.MCClientManager]: MC Client connected to cluster dev.
2021-08-26 15:21:05,241 [ INFO] [MC-Client-dev.event-1] [c.h.w.s.MCClientManager]: Started communication with member: Member [10.52.2.9]:5701 - f4df86c9-bc6b-4f62-b3e7-72b85fbbc74e
2021-08-26 15:21:05,245 [ INFO] [MC-Client-dev.event-1] [c.h.w.s.MCClientManager]: Started communication with member: Member [10.52.1.4]:5701 - 4c3b692e-0b56-4606-bf34-bd2b86c9f443
2021-08-26 15:21:05,251 [ INFO] [MC-Client-dev.event-1] [c.h.w.s.MCClientManager]: Started communication with member: Member [10.52.0.13]:5701 - 25cf88bb-f40b-40d5-b5a0-43c1da81b2bd
2021-08-26 15:21:07,234 [ INFO] [main] [c.h.w.Launcher]: Hazelcast Management Center successfully started at http://localhost:8080/

```

### Step 4: Clean up

To clean up your Kubernetes cluster execute the following commands.

```shell
kubectl delete -f config/samples/_v1alpha1_hazelcast.yaml
kubectl delete -f config/samples/_v1alpha1_managementcenter.yaml
make undeploy
kubectl delete secret hazelcast-license-key
```

## Running Tests

There are different types of tests related to Hazelcast Platform Operator.

### Running unit & integration tests
To run unit & integration tests, execute the following command.

```shell
make test
```

You can also run unit & integration tests separately by `make test-unit` and `make test-it` commands.

### Running end-to-end tests 

You need a Kubernetes cluster and your local kubectl context configured. You can run end-to-end tests by either deploying manager to cluster or running manager locally.

#### Deploy Manager to Cluster

Execute the following commands to run the end-to-end tests.

```shell
kubectl create ns <YOUR NAMESPACE>

make deploy NAMESPACE=<YOUR NAMESPACE> IMG=hazelcast/hazelcast-platform-operator:5-preview-snapshot

kubectl create secret generic hazelcast-license-key --namespace <YOUR NAMESPACE> --from-literal=license-key=<YOUR LICENSE KEY>

make test-e2e NAMESPACE=<YOUR NAMESPACE>
```

#### Run Manager Locally

Execute the following commands to run the end-to-end tests.

```shell
kubectl create ns <YOUR NAMESPACE>

make install run

kubectl create secret generic hazelcast-license-key --namespace <YOUR NAMESPACE> --from-literal=license-key=<YOUR LICENSE KEY>

RUN_MANAGER_LOCALLY=true make test-e2e NAMESPACE=<YOUR NAMESPACE>
```

## Check that the Hazelcast Cluster is Running

To check if a cluster is running, see the `status` field of the Hazelcast resource.

The status can be checked with `kubectl get hazelcast`:
```shell
NAME        STATUS    MEMBERS
hazelcast   Running   3/3
```

Or `kubectl get hazelcast -o=yaml` for the long format:
```yaml
status:
  hazelcastClusterStatus:
    readyMembers: 3/3
  phase: Running
```

The `phase` field represents the current status of the cluster, and can contain any of the following values:

* `Running`: The cluster is up and running.
* `Pending`: The cluster is in the process of starting.
* `Failed`: An error occurred while starting the cluster.

Any additional info such as validation errors will be provided in the `message` field.

The `readyMembers` field represents the number of Hazelcast members that are connected to the cluster.

> Note: Use the `readyMembers` field only for informational purposes. This field is not always accurate. Some members may have joined or left the cluster since this field was last updated.

## Running operator locally

Hazelcast Platform Operator uses `hazelcast go-client` to connect to the cluster.
For these reason the pods needs to be exposed outside the cluster.
Run `make expose-local` command that will expose Hazelcast member to `localhost:8000`.

The operator run must be build with `build constraint` tag `localrun`:

```shell
go build -o bin/manager -tags localrun main.go
```
Or using `make` that will include the tag by default:
```shell
make build
make run
```
You can override the build tags in the `make` commands by setting `GO_BUILD_TAGS` env variable.

> Note: Before running operator you need to run `make install` command to install the CRDs.

### Setting up build tags in GoLand

To run the operator from `GoLand` execute the following steps to add build tags:

1. In GoLand Preferences navigate to `Go | Build tags & Vendoring`
2. In the `Custom tags` field enter `localrun`
3. Go to the `Run configuration` of the `Go build` select the `Use all custom build tags` checkbox

Now you can run the `main.go` using `GoLand`.
