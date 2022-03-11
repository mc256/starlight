# Starlight: Fast Container Provisioning
Starlight speeds up deploying and updating containers to workers, 
while maintaining backwards compatibility with existing tools.
It so fast that it starts containers faster than merely downloading an optimized data package, 
yet with practically no overhead.

## We want to extend cloud practices further to the edge
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


## Citation
If you find Starlight useful in your work, please cite our NSDI 2022 paper:
```
@inproceedings{starlight,
author = {Jun Lin Chen and Daniyal Liaqat and Moshe Gabel and Eyal de Lara},
title = {Starlight: Fast Container Provisioning on the Edge and over the WAN },
booktitle = {19th USENIX Symposium on Networked Systems Design and Implementation (NSDI '22)},
year = {2022},
note = {To appear.}
}
```
---

## Get Started

Suppose you have a container that you want to deploy 
on a [standard **registry**](https://github.com/distribution/distribution).
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
<br>[Find out how to install **Starlight snapshotter plugin** & **Starlight CLI tool** ➡️](https://github.com/mc256/starlight/blob/master/docs/starlight-snapshotter.md)


4) Convert the container image to the **Starlight format** container image.
More specifically, the storage format of the compressed layers needs to be converted to the new format 
and then the layers stored in the registry. 
The new format is backwards compatible and almost the same size, 
so you don't need to store every layer twice.
In addition, the proxy needs some metadata about the list of files in the container to compute the delta bundle for deployment.
<br>
<br>The **Starlight CLI tool** features the image conversion, example:
<br>```ctr-starlight convert $MY_REGISTRY/redis:6.0.2 $MY_REGISTRY/redis:6.0.2-sl```


5) Collect traces on the worker for container startup. 
This entails starting the container on the worker while collecting file access traces 
that are sent to the proxy.
<br>
<br>The **Starlight CLI tool** features trace collection, example:
<br>```ctr-starlight pull $MY_REGISTRY/redis:6.0.2-starlight &&```
<br>```ctr-starlight create --optimize $MY_REGISTRY/redis:6.0.2-sl $MY_REGISTRY/redis:6.0.2-sl $MY_RUNTIME &&```
<br>```ctr task start $MY_RUNTIME```
<br> After finished running the container several times, then we can report all the traces to the proxy, using:
<br>```ctr-starlight report --server $MY_STARLIGHT_PROXY```

🙌 That's it ! You can now deploy the container to as many Starlight workers as you want, and it should be fast!

<br>

Note:

- step **2** must be done on each worker.
- steps **3** and **4** must be done for every container image you want to deploy using Starlight. 
The good news is that they should be quick, a few minutes for each container.