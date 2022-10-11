# Starlight Proxy Helm Package

## TL;DR

```shell
helm install my-starlight-proxy oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.1.1
```

## Prerequisites

The current deployment has tested in this environment:

- Kubernetes 1.24+
- Helm 3.9.0+
- PV provisioner support in the underlying infrastructure
- ReadWriteOnce volumes persistence storage


## Introductions

This chart bootstraps a **Starlight Proxy** deployment on a Kubernetes cluster.

It also comes with a **[Container Registry (v2)](https://github.com/distribution/distribution)** and a **[web-based container registry UI](https://github.com/Joxit/docker-registry-ui)** in the default deployment for convenience and can be disabled by setting the parameters.


## Install

Container registry requries persistence storage for storing the metadata.

```
kubectl patch storageclass local-path -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

To install the chart with the app name `my-starlight-proxy`:

```shell
helm install my-starlight-proxy oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.1.1
```


You may want to set a few parameter for example the domain name for the ingress, for example set the domain name for ingress to `mydomain.local`:

```shell
helm install my-starlight-proxy oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.1.1 \
--set "ingress.hosts={mydomain.local}"
```

Please check the Parameters section for more information.


## Uninstall

To uninstall the app  `my-starlight-proxy`:

```shell
helm delete my-starlight-proxy
```



## Parameters

### Common Parameters

| Name     | Description | Value|
| ---      | ---       | --- |
| ingress.hosts | domain name for the ingress | [starlight.lan] |
| registryAddress | customize registry address if choose not to use the container registry in this chart | null |
| registry.enable | enable container registry in this deployment | true |
| registryUi.enable | enable web-base UI container registry in this deployment | true |

### Starlight Proxy

| Name     | Description | Value|
| ---      | ---       | --- |
| starlightProxy.tag | tage of the image | "latest" |
| starlightProxy.pullPolicy | pull image policy | IfNotPresent |
| starlightProxy.persistence.enabled | tage of the image | "latest" |
| starlightProxy.persistence.existingClaim | if specified, use existing PV | "" |
| starlightProxy.persistence.storageClass | storage class, if not specified, used default storage class | "" |
| starlightProxy.persistence.size | storage size | 2Gi |

### Registry

| Name     | Description | Value|
| ---      | ---       | --- |
| registry.enable | enable registry | true |
| registry.repository | container image | "registry"|
| registry.pullPolicy | pull image policy | IfNotPresent |
| registry.tag | tage of the image | "latest" |
| registry.persistence.enabled | tage of the image | "latest" |
| registry.persistence.existingClaim | if specified, use existing PV | "" |
| registry.persistence.storageClass | storage class, if not specified, used default storage class | "" |
| registry.persistence.size | storage size | 2Gi |


