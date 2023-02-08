# Hazelcast Platform Operator #
[![GKE](https://img.shields.io/endpoint?url=https%3A%2F%2Fgist.githubusercontent.com%2FdevOpsHelm%2Fe513bc27d500818292261e4235723e5b%2Fraw%2FGKE.json%3Fcachebust%3D1)](https://hazelcast.github.io/hazelcast-platform-operator/gke)&nbsp; [![EKS](https://img.shields.io/endpoint?url=https%3A%2F%2Fgist.githubusercontent.com%2FdevOpsHelm%2Fe513bc27d500818292261e4235723e5b%2Fraw%2FEKS.json%3Fcachebust%3D1)](https://hazelcast.github.io/hazelcast-platform-operator/eks)&nbsp; [![AKS](https://img.shields.io/endpoint?url=https%3A%2F%2Fgist.githubusercontent.com%2FdevOpsHelm%2Fe513bc27d500818292261e4235723e5b%2Fraw%2FAKS.json%3Fcachebust%3D1)](https://hazelcast.github.io/hazelcast-platform-operator/aks)&nbsp; [![OCP](https://img.shields.io/endpoint?url=https%3A%2F%2Fgist.githubusercontent.com%2FdevOpsHelm%2Fe513bc27d500818292261e4235723e5b%2Fraw%2FOCP.json%3Fcachebust%3D1)](https://hazelcast.github.io/hazelcast-platform-operator/ocp)&nbsp; [![KinD](https://img.shields.io/github/actions/workflow/status/hazelcast/hazelcast-platform-operator/k8s-dist-tests.yaml?branch=main&label=KinD&style=flat&color=green)](https://hazelcast.github.io/hazelcast-platform-operator/pr)

<img align="right" src="https://hazelcast.com/brand-assets/files/hazelcast-stacked-flat-sm.png">

Easily deploy Hazelcast clusters and Management Center into Kubernetes/OpenShift environments and manage their lifecycles.

Hazelcast Platform Operator is based on the [Operator SDK](https://github.com/operator-framework/operator-sdk) to follow best practices.

Here is a short video to deploy a simple Hazelcast Platform cluster and Management Center via Hazelcast Platform Operator:

[Deploy a Cluster With the Hazelcast Platform Operator for Kubernetes](https://www.youtube.com/watch?v=4cK5I74nmr4)

## Table of Contents

* [Documentation](#documentation)
* [Features](#features)
* [Contribute](#contribute)
* [License](#license)

## Documentation

1. [Get started](https://docs.hazelcast.com/operator/latest/get-started) with the Operator
2. [Connect the cluster from outside Kubernetes](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-expose-externally)
3. [Restore a Cluster from Cloud Storage with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-external-backup-restore)
4. [Replicate Data between Two Hazelcast Clusters with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-wan-replication)
5. [Configure MongoDB Atlas as an External Data Store for the Cluster with Hazelcast Platform Operator](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-map-store-mongodb-atlas)

## Features

Hazelcast Platform Operator supports the features below:

* Custom resource for Hazelcast Platform (Open Source & Enterprise) and Management Center
* Observe status of Hazelcast clusters and Management Center instances
* High Availability Mode configuration to create clusters that are resilient to node and zone failures
* Scale up and down Hazelcast clusters
* Expose Hazelcast cluster to external
  clients ([Smart & Unisocket](https://docs.hazelcast.com/hazelcast/latest/clients/java#java-client-operation-modes))
* Backup Hazelcast persistence data to cloud storage with the possibility of scheduling it and restoring the data accordingly
* WAN Replication feature when you need to synchronize multiple Hazelcast clusters, which are connected by WANs
* User Code Deployment feature, which allows you to deploy custom and domain classes from cloud storages to Hazelcast members
* ExecutorService and EntryProcessor support
* Support several data structures like Map, Topic, MultiMap, ReplicatedMap, Queue and Cache which can be created dynamically via specific Custom Resources
* MapStore support for Map CR

For Hazelcast Platform Enterprise, you can request a trial license key from [here](https://trialrequest.hazelcast.com).

## Contribute

Before you contribute to the Hazelcast Platform Operator, please read the following:

* [Contributing to Hazelcast Platform Operator](CONTRIBUTING.md)
* [Developing and testing Hazelcast Platform Operator](DEVELOPER.md)
* [Hazelcast Platform Operator Architecture](ARCHITECTURE_OVERVIEW.md)

## License

Please see the [LICENSE](LICENSE) file.
