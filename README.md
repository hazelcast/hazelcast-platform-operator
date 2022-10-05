# Hazelcast Platform Operator #
[![Nightly E2E tests on GKE](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-gke.yaml/badge.svg?event=schedule)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-gke.yaml) [![Nightly E2E tests on AWS](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-eks.yaml/badge.svg?event=schedule)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-eks.yaml) [![Nightly E2E tests on Azure](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-aks.yaml/badge.svg?event=schedule)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-aks.yaml)

[![Nightly E2E tests on OCP](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-ocp.yaml/badge.svg?event=schedule)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-ocp.yaml) [![Nightly E2E tests on Kind](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-kind.yaml/badge.svg?event=schedule)](https://github.com/hazelcast/hazelcast-platform-operator/actions/workflows/nightly-e2e-kind.yaml)

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

1. [Get started](https://docs.hazelcast.com/operator/latest/get-started) with the Operator.
2. [Connect the cluster from outside Kubernetes](https://docs.hazelcast.com/tutorials/hazelcast-platform-operator-expose-externally)
   from the outside.

## Features

Hazelcast Platform Operator supports the features below:

* Custom resource for Hazelcast Platform (Open Source & Enterprise) and Management Center
* Observe status of Hazelcast clusters and Management Center instances
* Scale up and down Hazelcast clusters
* Expose Hazelcast cluster to external
  clients ([Smart & Unisocket](https://docs.hazelcast.com/hazelcast/latest/clients/java#java-client-operation-modes))

For Hazelcast Platform Enterprise, you can request a trial license key from [here](https://trialrequest.hazelcast.com).

## Contribute

Before you contribute to the Hazelcast Platform Operator, please read the following:

* [Contributing to Hazelcast Platform Operator](CONTRIBUTING.md)
* [Developing and testing Hazelcast Platform Operator](DEVELOPER.md)
* [Hazelcast Platform Operator Architecture](ARCHITECTURE_OVERVIEW.md)

## License

Please see the [LICENSE](LICENSE) file.
