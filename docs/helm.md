# Starlight Proxy Helm Package

## TL;DR

```shell
helm upgrade --install  -f starlight/values.yaml starlight oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.2.3
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
helm install my-starlight-proxy oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.2.3
```


You may want to set a few parameter for example the domain name for the ingress, for example set the domain name for ingress to `mydomain.local`:

```shell
helm install my-starlight-proxy oci://ghcr.io/mc256/starlight/starlight-proxy-chart --version 0.2.3 \
--set "ingress.hosts={mydomain.local}"
```

Please check the Parameters section for more information.


## Uninstall

To uninstall the app  `my-starlight-proxy`:

```shell
helm delete my-starlight-proxy
```



## Parameters
Please the comments in the [`values.yaml`]() file

