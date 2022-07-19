# Starlight: Fast Container Provisioning

[![Docker Image](https://github.com/mc256/starlight/actions/workflows/docker-image.yml/badge.svg)](https://github.com/mc256/starlight/actions/workflows/docker-image.yml)
[![Helm Chart](https://github.com/mc256/starlight/actions/workflows/helm-chart.yml/badge.svg)](https://github.com/mc256/starlight/actions/workflows/helm-chart.yml)

<img align="right" src="docs/provisioning-time-wan.png">

Starlight is an accelerator for provisioning container-based applications.
It speeds up deploying and updating containers on workers inside and outside the cloud, 
while maintaining backwards compatibility with existing tools.
It is so fast that containers can start faster than merely downloading an optimized data package, 
yet with practically no overhead. 

The image on the right compares time to download and start containers using containerd ("baseline"), [eStargz](https://github.com/containerd/stargz-snapshotter/blob/main/docs/estargz.md), Starlight, and the time it takes to download an optimized update package using wget. 
The registry is in North Virginia.
Top row shows time to deploy a container to an empty worker, and bottom row time to update the container to a later version.
Read our [NSDI 2022 paper](https://www.usenix.org/conference/nsdi22/presentation/chen-jun-lin) for more results.

### Extend cloud practices to the edge and WAN
Using containers to provision workers in high-latency or low-bandwidth environments can be tricky.
The time it takes to deploy software and start a container increases dramatically with latency, 
and increases at a higher rate than the equivalent time to simply download the data.
Outside the datacenter, where round-trip times are in the order of tens or hundreds of milliseconds, 
container provisioning can be several times slower than in the cloud, even when the network has reasonable bandwidth.

### Why is container provisiong slow?
The root cause for this slow provisioning time is the overall design of the provisioning pipeline: 
it is pull-based, designed around the stack-of-layers abstraction container images, 
and does not explicitly consider container updates.
For example, updating a Java application to fix the Log4j vulnerability usually requires re-downloading the entire container image, even though the updated Log4j library only takes a fraction of that space. 
This can make provisioning slower than it should be even inside cloud data centers.

### How do we address this?
Starlight decouples the mechanism of container provisioning from container development.
Starlight maintains the convenient stack-of-layers structure of container images, 
but uses a different representation when deploying them over the network.
The development and operational pipelines remain unchanged.
<br>[See how Starlight works ➡️](docs/starlight-workflow.md) or [read our NSDI 2022 paper](https://www.usenix.org/conference/nsdi22/presentation/chen-jun-lin).

## Architecture
Starlight is implemented on top of **containerd**. It it comprised of cloud and worker components.
* A **proxy** server on the cloud side mediates between Starlight workers and any standard registry server.
* On the worker side, a **command line tool** tells Starlight to PULL, CREATE, and START containers.
* A **Starlight snapshotter plugin** runs inside the containerd snapshotter dameon of each worker node. It receives user commands and implements them.
* Instead of OverlayFS, Starlight uses an efficient **FUSE-based filesystem** on worker nodes.

## Getting Started

[TL;DR?](docs/newbie.md)

Suppose you have a container on a [OCI compatible **registry**](https://github.com/distribution/distribution) that you want to deploy.
You need to:

1) Set up a **Starlight proxy**, 
ideally close to the **registry** server you are using. Configure the proxy server to point to the registry and run it.
Starlight supports any standard registry. (It can be deployed to k8s using ***Helm***)
<br>[Find out how to install **Starlight proxy** ➡️](docs/starlight-proxy.md) 


2) Set up the worker to be able to run Starlight. 
This involves 
installing **containerd** and the **Starlight snapshotter plugin**, 
configuring containerd to use the plugin, 
and starting the Starlight snapshotter daemon
(you also need to tell the snapshotter the address of the proxy server).
<br>[Find out how to install **containerd** & **Starlight snapshotter plugin** ➡️](docs/starlight-snapshotter.md)


3) Convert the container image to the **Starlight format** container image.
   More specifically, the storage format of the compressed layers needs to be converted to the Starlight format and then the layers stored in the registry. 
   The Starlight format is **backwards compatible** and almost the same size, so there is no need to store compressed layers twice. In other words, non-Starlight workers will descrompress Starlight images with no chanages.
   The **Starlight CLI tool** features the image conversion, example:
   ```shell
    ctr-starlight convert \
        --insecure-source --insecure-destination \
        $REGISTRY/redis:6.2.1 $REGISTRY/redis:6.2.1-starlight
   ```
   `$REGISTRY` is your container registry (e.g. `172.18.2.3:5000`).
   
   In addition, the proxy needs some metadata about the list of files in the container to compute the data for deployment.
   ```shell
   curl http://$STARLIGHT_PROXY/prepare/redis:6.2.1-starlight
   #Cached TOC: redis:6.2.1-starlight
   ```
   `$STARLIGHT_PROXY` is the address of your Starlight Proxy (e.g. `172.18.2.3:8090`)

4) Collect traces on the worker for container startup. 
   This entails starting the container on the worker while collecting file access traces that are sent to the proxy.
   
   The **Starlight CLI tool** features trace collection, example:
   ```shell
   sudo ctr-starlight pull redis:6.2.1-starlight && \
   mkdir /tmp/test-redis-data && \
   sudo ctr-starlight create --optimize \
          --mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
       --env-file ./demo/config/all.env \
       --net-host \
       redis:6.2.1-starlight \
       redis:6.2.1-starlight \
       instance1 && \
   sudo ctr task start instance1
   ```
   
   You may terminate the container using `Ctrl-C`, and remove the container:
   ```shell
   sudo ctr container rm instance1
   ```
   Traces will be saved to `/tmp/starlight-optimizer` folder.
   
   After finished running the container several times, then we can report all the traces to the proxy, using:
   ```shell
   ctr-starlight report --server $STARLIGHT_PROXY --plain-http
   ```

5) Reset `containerd` and `starlight`. Clean up all the downloaded containers and cache.
   ```shell
   sudo ./demo/reset.sh
   ```

🙌 That's it! You can now deploy the container to as many Starlight workers as you want, and it should be fast!

Note step **2** must be done on each worker, and steps **3** and **4** must be done for every container image you want to deploy using Starlight. 
The good news is that they should be quick, a few minutes for each container.

## Deploying containers

Start a container using Starlight
```shell
sudo ctr-starlight pull redis:6.2.1-starlight && \
mkdir /tmp/test-redis-data && \
sudo ctr-starlight create \
	--mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
	--env-file ./demo/config/all.env \
	--net-host \
	redis:6.2.1-starlight \
	redis:6.2.1-starlight \
    instance3 && \
sudo ctr task start instance3
```

Update a container using Starlight (Step 3 and Step 4 need to be done for `redis:6.2.2`)
```shell
sudo ctr-starlight pull redis:6.2.1-starlight redis:6.2.2-starlight && \
sudo ctr-starlight create \
	--mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
	--env-file ./demo/config/all.env \
	--net-host \
	redis:6.2.2-starlight \
	redis:6.2.2-starlight \
    instance4 && \
sudo ctr task start instance4
```

For more information, please check out `ctr-starlight --help` and `starlight-grpc --help`

## Citation
If you find Starlight useful in your work, please cite our NSDI 2022 paper:
```bibtex
@inproceedings {chen2022starlight,
author = {Jun Lin Chen and Daniyal Liaqat and Moshe Gabel and Eyal de Lara},
title = {Starlight: Fast Container Provisioning on the Edge and over the {WAN}},
booktitle = {19th USENIX Symposium on Networked Systems Design and Implementation (NSDI 22)},
year = {2022},
address = {Renton, WA},
url = {https://www.usenix.org/conference/nsdi22/presentation/chen-jun-lin},
publisher = {USENIX Association},
month = apr,
}
```

## Roadmap
Starlight is not complete. On our roadmap:

* Authentication with Docker Hub.
* Integration with Kubernetes.
* Supporting docker-compose.
* Jointly optimizing multiple container deployments to same worker.
* Proxy could request partial files from the registry, rather than entire layers.
* Converting containers that have already been fully retrieved using Starlight to use OverlayFS.
