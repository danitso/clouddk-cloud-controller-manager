# Kubernetes Cloud Controller Manager for Cloud.dk
`clouddk-cloud-controller-manager` is a Kubernetes Cloud Controller Manager implementation (or out-of-tree cloud-provider) for [Cloud.dk](https://cloud.dk).

> **WARNING:** This project is under active development and should be considered alpha.

## Introduction
External cloud providers were introduced as an _Alpha_ feature in Kubernetes 1.6 with the addition of the [Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/) binary. External cloud providers are Kubernetes (master) controllers that implement the cloud-provider specific control loops required for Kubernetes to function.

`clouddk-cloud-controller-manager` is one such provider and is designed to work with Kubernetes clusters running on [Cloud.dk](https://cloud.dk). It enables these clusters to retrieve metadata for nodes and create services of type `LoadBalancer`.
