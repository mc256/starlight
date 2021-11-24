# Starlight: Fast Container Provisioning on the Edge and over the WAN

We want to extend cloud practices further to the edge! Starlight speeds up the container deployment on the edge. 


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