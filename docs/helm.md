# Starlight Proxy Helm Package

## TL;DR

```shell
helm upgrade --install -f starlight/values.yaml \
     starlight \
     oci://ghcr.io/mc256/starlight/starlight \
     --version 0.2.3
```

## Prerequisites

The current deployment has tested in this environment:

- Kubernetes 1.24+
- Helm 3.9.0+
- PV provisioner support in the underlying infrastructure
- ReadWriteOnce volumes persistence storage
- Linux kernel 5.15.0+


## Introductions

This chart bootstraps a **Starlight Proxy** deployment on a Kubernetes cluster.

It also comes with a **[Container Registry (v2)](https://github.com/distribution/distribution)**, 
**[a PostgresQL as the metadata database](https://www.postgresql.org/)** 
and **[Adminer for managing the database](https://www.adminer.org/)** 
in the default deployment for convenience and can be disabled by setting the parameters.


## Install

Container registry requries persistence storage for storing the metadata. 
In this case, we use the [Local Path Provisioner](https://github.com/rancher/local-path-provisioner) 

```shell
kubectl apply -f \
  https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.23/deploy/local-path-storage.yaml
  
kubectl patch storageclass local-path \
  -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
```

To install the chart with the app name `my-starlight-proxy`:

```shell
helm install my-starlight-proxy \
  oci://ghcr.io/mc256/starlight/starlight \
  --version 0.2.3
```


You may want to set a few parameter for example the domain name for the ingress, for example set the domain name for ingress to `mydomain.local`:

```shell
helm install my-starlight-proxy \
  oci://ghcr.io/mc256/starlight/starlight \
  --version 0.2.3 \
  --set "ingress.hosts={mydomain.local}"
```

Please check the Parameters section for more information.


## Uninstall

To uninstall the app  `my-starlight-proxy`:

```shell
helm delete my-starlight-proxy
```


## Parameters
Please see the comments in the [`values.yaml`](https://github.com/mc256/starlight/blob/master/demo/chart/values.yaml) file.


## How to use Starlight on the edge?

We can use Starlight to pull container image in the `initContainers`. 
The Starlight CLI talks to the Starlight Daemon via gRPC socket to request the image
```yaml
  initContainers:
  - name: init-redis
    image: ghcr.io/mc256/starlight/cli:latest
    command:
    - /opt/ctr-starlight  
    - pull 
    - --profile
    - xxx
    - harbor.yuri.moe/x/redis:6.2.7
    env:
    - name: CONTAINERD_NAMESPACE
      value: "k8s.io"
    volumeMounts:
    - name: socket
      mountPath: /run/starlight
```

The following is a complete example of the a Redis deployment:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-redis
spec:
  selector:
    matchLabels:
      app: test-redis
  template:
    metadata:
      labels:
        app: test-redis
    spec:
      volumes:
      - name: socket
        hostPath:
          path: /run/starlight
      - name: redis-pvc
        persistentVolumeClaim:
          claimName: redis-pvc
      initContainers:
      - name: init-redis
        image: ghcr.io/mc256/starlight/cli:latest
        #command:  ["/bin/sh", "-ec", "while :; do echo '.'; sleep 60 ; done"]
        command:
        - /opt/ctr-starlight  
        - pull 
        - --profile
        - xxx
        - harbor.yuri.moe/x/redis:6.2.7
        env:
        - name: CONTAINERD_NAMESPACE
          value: "k8s.io"
        volumeMounts:
        - name: socket
          mountPath: /run/starlight
      containers:
      - name: test-redis
        image: harbor.yuri.moe/x/redis:6.2.7
        securityContext:
          runAsUser: 999
          allowPrivilegeEscalation: false
        resources:
          limits:
            memory: "128Mi"
            cpu: "500m"
        ports:
        - containerPort: 6379
        volumeMounts:
        - mountPath: /data
          name: redis-pvc
          subPath: redis
          
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  labels:
    io.kompose.service: redis-pvc
  name: redis-pvc
spec:
  storageClassName: "local-path"
  accessModes:
    - ReadWriteOnce
  volumeMode: Filesystem
  resources:
    requests:
      storage: 256Mi
```

Having questions? Please [create an issue here](https://github.com/mc256/starlight/issues/new?assignees=m256&labels=&template=question.md&title=)