# Hazelcast Enterprise Operator

## Quick Start

### Prerequisites

Kubernetes cluster (with admin rights), and the kubectl command configured.

### Step 1: Create Secret with Hazelcast License Key

```shell
kubectl create secret generic hazelcast-license-key --from-literal=license-key=<YOUR LICENSE KEY>
```

### Step 2: Start Hazelcast Enterprise Operator

```shell
git clone git@github.com:hazelcast/hazelcast-enterprise-operator.git
cd hazelcast-enterprise-operator
make deploy IMG=hazelcast/hazelcast-enterprise-operator:5-preview-snapshot
```

Note: If you want to run the operator locally, you can execute `make install run` instead of `make deploy`.

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

You can check the operator's logs to see the resource creation logs:
```
$ kubectl logs deployment.apps/hazelcast-enterprise-controller-manager manager
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

### Step 4: Clean up

To clean up your Kubernetes cluster execute the following commands.

```shell
kubectl delete -f config/samples/_v1alpha1_hazelcast.yaml
make undeploy
kubectl delete secret hazelcast-license-key
```

## Running Tests

There are different types of tests related to Hazelcast Enterprise Operator.

### Running unit & integration tests
To run unit & integration tests, execute the following command.

```shell
make test
```

### Running end-to-end tests 

You need a Kubernetes cluster and also your local kubectl context configured. Then, execute the following commands.

```shell
kubectl create ns <YOUR NAMESPACE>

make deploy NAMESPACE=<YOUR NAMESPACE> IMG=hazelcast/hazelcast-enterprise-operator:5-preview-snapshot

kubectl create secret generic hazelcast-license-key --namespace <YOUR NAMESPACE> --from-literal=license-key=<YOUR LICENSE KEY>

make test-e2e NAMESPACE=<YOUR NAMESPACE>
```

