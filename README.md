# Starlight: Fast Container Provisioning
Starlight speeds up deploying and updating containers to workers, while maintaining backwards compatibility with existing tools.
It so fast that it starts containers faster than merely downloading an optimized data package, yet with practically no overhead.

## We want to extend cloud practices further to the edge
Using containers to provision workers in high latency environments is often tricky.
The time it takes to deploy software and start a container increase dramatically with latency, and increase at a higher rate than the equivalent time to simply download the data.
Outside the datacenter, where round-trip times are in the order of tens or hundreds of milliseconds, container provisioning can be several times higher than in the cloud, even when the network has reasoinable bandwidth.
The root cause for this slow provisioning time is the overall design of the provisioning pipeline: it is pull-based, designed around the stack-of-layers abstraction container images, and does not explicitly consider container updates. 

Starlight is an accelerator for provisioning container-based applications that decouples the mechanism of container provisioning from container development.
Starlight maintains the convenient stack-of-layers structure of container images, but uses a different representation when deploying them over the network.
The development and operational pipelines remain unchanged: users can use existing containers, tools, and registries. 
On average, Starlight provisioning is 3 times faster than the current containerd implementation, and almost 2 times faster than eStargz.
Starlight improves provisioning time inside the cloud as well: for example it can deploy updates much faster than standard containerd and eStargz.
Happily, Starlight has little-to-no runtime overhead: its worker performance matches the standard containerd.

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

Suppose you have a container you on some standard registry server that you want to deploy.
You need to:

1) Set up a Starlight proxy server, ideally close to the registry server you are using. Configure the proxy server to point to the registry and run it.
Starlight supports any standard registry, but some features may not be ready yet like Docker Hub authentication.

2) Set up the worker to be able to run Starlight. This involves installing containerd and the Starlight plugin, configuring containerd to use the plugin, and starting the Starlight snapshotter daemon (you also need to tell the snapshotter the address of the proxy server).

3) Convert the container image to the Starlight format.
More specifically, the storage format of the compressed layers needs to be converted to the new format and then the layers stored in the registry. The new format is backwards compatible and almost the same size, so you don't need to store every layer twice.
In addition, the proxy server needs some metadata about the list of files in the container.
There is (supposed to be) an converter script/command that does all this easily and automatic.

4) Collect traces on the worker for container startup. This entails starting the container on the worker while collecting file access traces that are sent to the proxy.
There is supposed to be a command to do this automatically, but it currently may or may not require a special branch of Starlight

DONE! You can now deploy the container to as many Starlight workers as you want, and it should be fast!

Note steps 3 and 4 must be done for every container image you want to deploy using Starlight. The good news is that they should be quick, a few minutes for each container, and only done for each container (*not* once per worker!).