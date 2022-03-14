# Starlight: Fast Container Provisioning
Starlight speeds up deploying and updating containers to workers, 
while maintaining backwards compatibility with existing tools.
It so fast that it starts containers faster than merely downloading an optimized data package, 
yet with practically no overhead.

## Extend cloud practices further to the edge
Using containers to provision workers in high latency environments is often tricky.
The time it takes to deploy software and start a container increase dramatically with latency, 
and increase at a higher rate than the equivalent time to simply download the data.
Outside the datacenter, where round-trip times are in the order of tens or hundreds of milliseconds, 
container provisioning can be several times higher than in the cloud, even when the network has reasonable bandwidth.
The root cause for this slow provisioning time is the overall design of the provisioning pipeline: 
it is pull-based, designed around the stack-of-layers abstraction container images, 
and does not explicitly consider container updates 
(For example: Log4j library only takes a few MB, 
but a JAVA application might be huge. 
Fixing the Log4j security issue usually requires re-downloading the entire container image). 

Starlight is an accelerator for provisioning container-based applications that 
decouples the mechanism of container provisioning from container development.
Starlight maintains the convenient stack-of-layers structure of container images, 
but uses a different representation when deploying them over the network.
The development and operational pipelines remain unchanged.

## Workflow Overview

![starlight-workflow](docs/starlight-workflow.png)

Once the user issues a worker `PULL` command to download a set of containers ①,
the command is received by the standard **containerd** daemon.
**containerd** then forwards the command to the **Starlight snapshotter** daemon ②, 
and waits for confirmation that the requested images have been found.
The Starlight snapshotter opens a connection to the **Starlight proxy** 
and sends the list of requested containers as well as the list of relevant containers that already exist on the worker ③. 
The proxy queries the directory database ④ for the list of files in the various layers of the 
requested container image, as well in the image already available on the worker.

The proxy will then begin computing the **delta bundle** that includes the set of distinct compressed file contents that the worker does not already have, specifically organized to speed up deployment;
In the background, the proxy also responds with HTTP 200 OK header to the snapshotter, which notifies **containerd** that the `PULL` phase has finished successfully; the snapshotter however, remains active and keeps the connection open to receive the data from the proxy.
In the background, the proxy issues a series of requests to the registry ⑦ to retrieve the compressed contents of files needed for delta bundle.
Once the contents of the delta bundle has been computed, the proxy creates a **Starlight manifest** (SLM) -- the list of file metadata, container manifests, and other required metadata -- and sends it to the snapshotter ⑤,
which notifies **containerd** that the `PULL` phase has finished successfully.

## Get Started

Suppose you have a container that you want to deploy 
on a [OCI compatible **registry**](https://github.com/distribution/distribution).
You need to:

1) Set up a **Starlight proxy**, 
ideally close to the **registry** server you are using. Configure the proxy server to point to the registry and run it.
Starlight supports any standard registry.
<br>[Find out how to install **Starlight proxy** ➡️](https://github.com/mc256/starlight/blob/master/docs/starlight-proxy.md) 


2) Set up the worker to be able to run Starlight. 
This involves 
installing **containerd** and the **Starlight snapshotter plugin**, 
configuring containerd to use the plugin, 
and starting the Starlight snapshotter daemon
(you also need to tell the snapshotter the address of the proxy server).
<br>[Find out how to install **containerd** ➡️](https://containerd.io/downloads/)
<br>[Find out how to install **Starlight snapshotter plugin** & **Starlight CLI tool** ➡️](https://github.com/mc256/starlight/blob/master/docs/starlight-snapshotter-cli.md)


4) Convert the container image to the **Starlight format** container image.
More specifically, the storage format of the compressed layers needs to be converted to the new format 
and then the layers stored in the registry. 
The new format is backwards compatible and almost the same size, 
so you don't need to store every layer twice.
In addition, the proxy needs some metadata about the list of files in the container to compute the delta bundle for deployment.
<br>The **Starlight CLI tool** features the image conversion, example:
<br>```ctr-starlight convert $MY_REGISTRY/redis:6.2.1 $MY_REGISTRY/redis:6.2.1-starlight```
<br>(`$MY_REGISTRY` will be the server that runs container registry, for example, `gcr.io`)

5) Collect traces on the worker for container startup. 
This entails starting the container on the worker while collecting file access traces 
that are sent to the proxy.
<br>The **Starlight CLI tool** features trace collection, example:
```shell
ctr-starlight pull redis:6.2.1-starlight && \
mkdir /tmp/test-redis-data && \
sudo ctr-starlight create --optimize \
	--mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
	--env-file ./demo/config/all.env \
	--net-host \
	redis:6.2.1-starlight \
	redis:6.2.1-starlight \
    $MY_RUNTIME && \
ctr task start $MY_RUNTIME
```
(`$MY_RUNTIME` can be any string)

<br> After finished running the container several times, then we can report all the traces to the proxy, using:

```shell
ctr-starlight report --server $MY_STARLIGHT_PROXY
```
(`$MY_STARLIGHT_PROXY` will be the server that runs Starlight proxy, for example, `192.168.1.3`)

---

🙌 That's it ! You can now deploy the container to as many Starlight workers as you want, and it should be fast!

---


6) Reset `containerd` and `starlight`. Clean up all the downloaded containers and cache.
```shell
sudo systemctl stop containerd && \
sudo systemctl stop starlight && \
sudo rm -rf /var/lib/starlight-grpc && \
sudo rm -rf /var/lib/containerd && \
sudo rm -rf /tmp/test-redis-data && \
sudo systemctl start containerd && \
sudo systemctl start starlight 
```


7) Start the container using Starlight
```shell
ctr-starlight pull redis:6.2.1-starlight && \
mkdir /tmp/test-redis-data && \
sudo ctr-starlight create \
	--mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
	--env-file ./demo/config/all.env \
	--net-host \
	redis:6.2.1-starlight \
	redis:6.2.1-starlight \
    $MY_RUNTIME && \
ctr task start $MY_RUNTIME
```


8) Update the container using Starlight
```shell
ctr-starlight pull redis:6.2.1-starlight redis:6.2.2-starlight && \
mkdir /tmp/test-redis-data && \
sudo ctr-starlight create \
	--mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
	--env-file ./demo/config/all.env \
	--net-host \
	redis:6.2.2-starlight \
	redis:6.2.2-starlight \
    $MY_RUNTIME_2 && \
ctr task start $MY_RUNTIME_2
```

<br>

Note:

- step **2** must be done on each worker.
- steps **3** and **4** must be done for every container image you want to deploy using Starlight. 
The good news is that they should be quick, a few minutes for each container.


## Citation
If you find Starlight useful in your work, please cite our NSDI 2022 paper:
```bibtex
@inproceedings{starlight,
author = {Jun Lin Chen and Daniyal Liaqat and Moshe Gabel and Eyal de Lara},
title = {Starlight: Fast Container Provisioning on the Edge and over the WAN },
booktitle = {19th USENIX Symposium on Networked Systems Design and Implementation (NSDI '22)},
year = {2022},
note = {To appear.}
}
```
