# Contributing to Hazelcast

Hazelcast Platform Operator is Open Source software licensed under [Apache 2.0. license](LICENSE).
The main benefit of Open Source is that you don't need to wait for a vendor to provide a fix or a feature.
If you've got the skills (and the will), it's already at your fingertips.

There are multiple ways to contribute:

1. [Reporting an issue](#issue-reports)
2. [Sending a pull request](#pull-requests).
   Note that you don't need to be a developer to help us.
   Contributions that improve the documentation are always appreciated.

If you need assistance, please reach us directly via [Slack](https://slack.hazelcast.com/).

## Issue Reports

Thanks for reporting your issue.
To help us resolve your issue quickly and efficiently, we need as much data for diagnostics as possible.
Please share with us the following information:

1.	Exact operator version that you use (_e.g._ `5.0.1`, also whether it is a minor release or the latest snapshot).
2.	Hazelcast and ManagementCenter CR (Custom Resource) YAML files.
3.	Kubernetes distribution and version. (_e.g._ `GKE 1.21`, `Openshift 4.9` or `minikube`)
4.	Logs from the operator, Hazelcast and, Management Center pods.
5.	Detailed description of the steps to reproduce your issue.

## Pull requests

Thanks a lot for creating your PR!

A PR can target many different subjects:

* [Documentation](https://github.com/hazelcast/hazelcast-platform-operator-docs):
  either fix typos or improve the documentation as a whole
* Fix a bug
* Add a feature
* Anything else that makes Hazelcast Platform Operator better!

All PRs follow the same process:

1.	Contributions are submitted, reviewed, and accepted using the PR system on GitHub.
2.	For first-time contributors, our bot will automatically ask you to sign the Hazelcast Contributor Agreement([CLA](https://www.hazelcast.com/legal)) on the PR.
3.	The latest changes are in the `main` branch.
4.	Make sure to design clean commits that are easily readable. That includes descriptive commit messages.
5.  Please keep your PRs as small as possible, _i.e._ if you plan to perform a huge change, do not submit a single and large PR for it. For an enhancement or larger feature, you can create a GitHub issue first to discuss.
6.  Run make update-chart-crds command, if you modify the API of Custom Resources.
7.  Before you push, run the command `make lint` in your terminal and fix the linter errors if any. Push your PR once it is free of linter errors.
8.  If you submit a PR as the solution to a specific issue, please mention the issue number either in the PR description or commit message.