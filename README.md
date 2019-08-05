# Kubernetes Cloud Controller Manager for Cloud.dk
`clouddk-cloud-controller-manager` is a Kubernetes Cloud Controller Manager implementation (or out-of-tree cloud-provider) for [Cloud.dk](https://cloud.dk).

> **WARNING:** This project is under active development and should be considered alpha.

## Introduction
External cloud providers were introduced as an _Alpha_ feature in Kubernetes 1.6 with the addition of the [Cloud Controller Manager](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/) binary. External cloud providers are Kubernetes (master) controllers that implement the cloud-provider specific control loops required for Kubernetes to function.

`clouddk-cloud-controller-manager` is one such provider and is designed to work with Kubernetes clusters running on [Cloud.dk](https://cloud.dk). It enables these clusters to retrieve metadata for nodes and create services of type `LoadBalancer`.

## Preparation
In order to enable support for the controller, a flag must be set on the `kube-controller-manager` component.

In case the cluster was deployed with `kubeadm`, you must edit `/etc/kubernetes/manifests/kube-controller-manager.yaml` and add `--cloud-provider external` to the command section.

Alternatively, you can add the following fragment to a `kubeadm` configuration file:

```yaml
controllerManagerExtraArgs:
  cloud-provider: external
```

## Installation
Follow these simple steps in order to install the controller:

1. Ensure that `kubectl` is configured to reach your cluster

1. Retrieve the API key from [https://my.cloud.dk/account/api-key](https://my.cloud.dk/account/api-key)

1. Create a new SSH key pair

    ```bash
    rm -f /tmp/clouddk_ssh_key* \
        && ssh-keygen -b 4096 -t rsa -f /tmp/clouddk_ssh_key -q -N "" \
        && echo "CLOUDDK_SSH_PRIVATE_KEY: '$(cat /tmp/clouddk_ssh_key | base64 | tr -d '\n')'" \
        && echo "CLOUDDK_SSH_PUBLIC_KEY: '$(cat /tmp/clouddk_ssh_key.pub | base64 | tr -d '\n')'"
    ```

1. Create a new file called `config.yaml` with the following contents:

    ```yaml
    apiVersion: v1
    kind: Secret
    metadata:
      name: clouddk-cloud-controller-manager-config
      namespace: kube-system
    type: Opaque
    data:
      CLOUDDK_API_ENDPOINT: 'https://api.cloud.dk/v1'
      CLOUDDK_API_KEY: 'the API key retrieved in step 2'
      CLOUDDK_SSH_PRIVATE_KEY: 'the private SSH key generated in step 3'
      CLOUDDK_SSH_PUBLIC_KEY: 'the public SSH key generated in step 3'
    ```

1. Create the secret in `config.yaml` using `kubectl`

    ```bash
    kubectl apply -f ./config.yaml
    ```

1. Deploy the controller using `kubectl`

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/danitso/clouddk-cloud-controller-manager/master/deployment.yaml
    ```

    _It may be necessary to download the file and modify it before deploying the controller, if the default cluster settings do not match the settings of a particular cluster._

1. Verify that `clouddk-cloud-controller-manager` pods are being created and wait for them to reach a `Running` state

    ```bash
    kubectl get pods -l k8s-app=clouddk-cloud-controller-manager -n kube-system
    ```
