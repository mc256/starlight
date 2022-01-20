# Starlight: Fast Container Provisioning on the Edge and over the WAN

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

## Starlight Proxy


### Prebuild Docker Image and Docker-Compose

The prebuild image is available in 

```url
registry.yuri.moe/starlight-proxy:latest
```

Please first clone this project and go to the proxy demo folder
```shell
git clone git@github.com:mc256/starlight.git
cd ./starlight/demo/proxy
```

Update the registry address in `config.env` and then launch the proxy using `docker-compose up -d`




---

## Starlight Snapshotter

### 1. Prerequisites

Starlight depends on `containerd`. Please install `containerd` follow the instructions on [containerd's Github repository](https://github.com/containerd/containerd).

To enable the Starlight snapshotter plugin, add the following configuration to `/etc/containerd/config.toml`

```yaml
[proxy_plugins]
  [proxy_plugins.starlight]
    type = "snapshot"
    address = "/run/starlight-grpc/starlight-snapshotter.socket"
```

### 2. Build From Source

Checkout this repository

```shell
git clone git@github.com:mc256/starlight.git
```

Build and install this project

```shell
make && sudo make install
```


### 3. Run this project

```shell
starlight-grpc run --server $STARLIGHT_PROXY_ADDRESS --socket /run/starlight-grpc/starlight-snapshotter.socket
```

---

## Other configurations

Latency can impact the available bandwith if the TCP window is too small.
Please use the following configurations in `/etc/sysctl.conf` to increase the default TCP window size or compute a suitable configuration using https://www.speedguide.net/bdp.php.

```shell
net.core.wmem_max=125829120
net.core.rmem_max=125829120
net.ipv4.tcp_rmem= 10240 87380 125829120
net.ipv4.tcp_wmem= 10240 87380 125829120
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_sack = 1
net.ipv4.tcp_no_metrics_save = 1
net.core.netdev_max_backlog = 10000
```

After setting the new TCP window in `/etc/sysctl.conf`, use `sysctl -p` to apply the configuration.
