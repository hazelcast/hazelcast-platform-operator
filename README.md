# Hazelcast Enterprise Operator

## Local Quick Start

### Prerequisites

Kubernetes cluster (with admin rights), and the kubectl command configured.

### Step 1: Create Secret with Hazelcast License Key

```shell
kubectl create secret generic hazelcast-license-key --from-literal=license-key=<YOUR LICENSE KEY>
```

### Step 2: Start Operator Locally

You need to clone this repository before start operator container:

```shell
git clone git@github.com:hazelcast/hazelcast-enterprise-operator.git
cd hazelcast-enterprise-operator
```

Then run below make command to deploy required resources like CRD, RBAC and run operator locally:  
```shell
make install run
```
Output:
```
hazelcast-enterprise-operator/bin/controller-gen "crd:trivialVersions=true,preserveUnknownFields=false" rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
hazelcast-enterprise-operator/bin/kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/hazelcasts.hazelcast.com configured
hazelcast-enterprise-operator/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
controllers/hazelcast_resources.go
go vet ./...
go run ./main.go
I0616 16:28:05.675445   41482 request.go:655] Throttling request took 1.001012789s, request: GET:https://34.122.123.43/apis/autoscaling.k8s.io/v1?timeout=32s
2021-06-16T16:28:06.035+0300    INFO    controller-runtime.metrics      metrics server is starting to listen    {"addr": ":8080"}
2021-06-16T16:28:06.035+0300    INFO    setup   starting manager
2021-06-16T16:28:06.036+0300    INFO    controller-runtime.manager      starting metrics server {"path": "/metrics"}
2021-06-16T16:28:06.036+0300    INFO    controller-runtime.manager.controller.hazelcast Starting EventSource    {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "source": "kind source: /, Kind="}
2021-06-16T16:28:06.336+0300    INFO    controller-runtime.manager.controller.hazelcast Starting EventSource    {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "source": "kind source: /, Kind="}
2021-06-16T16:28:06.537+0300    INFO    controller-runtime.manager.controller.hazelcast Starting EventSource    {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "source": "kind source: /, Kind="}
2021-06-16T16:28:06.738+0300    INFO    controller-runtime.manager.controller.hazelcast Starting EventSource    {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "source": "kind source: /, Kind="}
2021-06-16T16:28:07.142+0300    INFO    controller-runtime.manager.controller.hazelcast Starting EventSource    {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "source": "kind source: /, Kind="}
2021-06-16T16:28:07.446+0300    INFO    controller-runtime.manager.controller.hazelcast Starting Controller     {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast"}
2021-06-16T16:28:07.447+0300    INFO    controller-runtime.manager.controller.hazelcast Starting workers        {"reconciler group": "hazelcast.com", "reconciler kind": "Hazelcast", "worker count": 1}
```

### Step 3: Start Hazelcast Enterprise Cluster

Run the following command to start Hazelcast Enterprise Cluster via applying CR yaml:

```shell
kubectl apply -f config/samples/_v1alpha1_hazelcast.yaml
```
```yaml
apiVersion: hazelcast.com/v1alpha1
kind: Hazelcast
metadata:
  name: hazelcast-sample
spec:
  clusterSize: 3
  repository: 'docker.io/hazelcast/hazelcast-enterprise'
  version: '5.0-SNAPSHOT'
  licenseKeySecret: hazelcast-license-key
```

Check your local operator's logs again, you will see resource creation logs:
```
...
2021-06-16T16:37:09.539+0300    DEBUG   controllers.Hazelcast   Finalizer added into custom resource successfully       {"hazelcast": "default/hazelcast-sample"}
2021-06-16T16:37:09.743+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast-sample", "ClusterRole": "hazelcast-sample", "result": "created"}
2021-06-16T16:37:09.932+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast-sample", "ServiceAccount": "hazelcast-sample", "result": "created"}
2021-06-16T16:37:10.370+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast-sample", "RoleBinding": "hazelcast-sample", "result": "created"}
2021-06-16T16:37:10.992+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast-sample", "Statefulset": "hazelcast-sample", "result": "created"}
2021-06-16T16:37:11.291+0300    DEBUG   controllers.Hazelcast   Statefulset resource version has been changed during create/update process.     {"hazelcast": "default/hazelcast-sample"}
2021-06-16T16:37:11.559+0300    INFO    controllers.Hazelcast   Operation result        {"hazelcast": "default/hazelcast-sample", "Statefulset": "hazelcast-sample", "result": "updated"}
```

Check Hazelcast member's log:
```shell
$ kubectl logs hazelcast-sample-0

....
Members {size:3, ver:3} [
        Member [10.131.0.130]:5701 - f447da35-0c04-4199-b99d-cb02376ad175 this
        Member [10.131.0.194]:5701 - c6b34055-3937-4adf-b0ae-d30f44495331
        Member [10.131.1.2]:5701 - 0fa2a21a-d159-4cac-ab08-bdac994d912a
]
```
