# Starlight Snapshotter Plugin

This is the **Step 2** to use Starlight:

Set up the worker to be able to run Starlight. 
This involves 
installing **containerd** and the **Starlight snapshotter plugin**, 
configuring containerd to use the plugin, 
and starting the Starlight snapshotter daemon
(you also need to tell the snapshotter the address of the proxy server).

[⬅️ Back to README.md](https://github.com/mc256/starlight)

---

## Method 1. Install Pre-built Package (Recommended)

Pre-build deb package is available for `amd64`, `armhf`, and `arm64`.

### 1. Install Starlight Snapshotter

Download and install the `.deb` package from the [release page](https://github.com/mc256/starlight/releases).

```shell
export ARCH=amd64
export SL_VERSION=0.1.1
wget "https://github.com/mc256/starlight/releases/download/v${SL_VERSION}/starlight-snapshotter_${SL_VERSION}_$ARCH.deb"
sudo apt install -f "./starlight-snapshotter_${SL_VERSION}_$ARCH.deb"
```

Update systemd service file `/lib/systemd/system/starlight-snapshotter.service`. 
- Change `STARLIGHT_PROXY` to the address of the Starlight Proxy.
- remove `--plain-http` if the Starlight Proxy is behind a HTTPS reverse proxy.
```
ExecStart=/usr/bin/starlight-grpc run --plain-http starlight.lan
```

Reload systemd service
```shell
sudo systemctl daemon-reload
sudo systemctl restart starlight-snapshotter
```

### 2. Configure `contaienrd`

Add configuration to `/etc/containerd/config.toml`. 
(If you have set other `proxy_plugins`, please manually edit the file)
```shell
sudo mkdir /etc/containerd/ && \
cat <<EOT | sudo tee -a /etc/containerd/config.toml > /dev/null
[proxy_plugins]
  [proxy_plugins.starlight]
    type = "snapshot"
    address = "/run/starlight-grpc/starlight-snapshotter.socket"
EOT
```

Restart `containerd` service
```shell
sudo systemctl restart containerd
```

Verify the Starlight snapshotter plugin is functioning
```shell
sudo ctr plugin ls | grep starlight 
# io.containerd.snapshotter.v1    starlight                -              ok
```


🙌 That's it. Please proceed to the **Step 3**.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started) 



---



## Method 2. Build from source

### 1. Install Dependencies
 
The worker machine is supposed to be far away (in latency) to the registry and proxy.
Please install **containerd** and **Starlight snapshotter** on a new machine (or VM), not the same machine that runs the proxy or the registry. 

The worker machine needs `build-essential` and `containerd`.
```shell
sudo apt update && sudo apt upgrade -y && \
sudo apt install -y build-essential containerd
```

Enable `containerd`
```shell
sudo systemctl enable containerd  && \
sudo systemctl start containerd
```

Verify `containerd` is running
```shell
sudo systemctl status containerd
#      Active: active
```

Install Go https://go.dev/doc/install ➡️
```shell
wget https://go.dev/dl/go1.17.8.linux-amd64.tar.gz && \
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
```

Add Go to the environment variable (You may want to change `.zshrc` or `.bashrc` file to permanently add this folder to the `PATH` environment variable)
```shell
export PATH=$PATH:/usr/local/go/bin
```

Verify Go is available
```shell
go version
# go version go1.17.8 linux/amd64
```

### 2. Clone and Build
Clone the Starlight repository
```shell
git clone https://github.com/mc256/starlight.git && \
cd starlight
```

Build the snapshotter plugin and CLI tool
```shell
make build-starlight-grpc build-ctr-starlight
```

### 3. Configure Starlight Snapshotter

You need to find out the IP address / DNS of the Starlight Proxy server (in **Step 1**. [Find out how to install **Starlight proxy** ➡️](docs/starlight-proxy.md) )

```shell
# This is an example
export STARLIGHT_PROXY=172.18.1.3:8090
export REGISTRY=172.18.1.3:5000
```

Verify that the Starlight proxy is accessible from the worker. 
```shell
curl http://$STARLIGHT_PROXY
# Starlight Proxy OK!
```

Install Starlight Snapshotter `systemd` service and CLI tool.
Please follow the prompt, enter 
```shell
sudo make install install-systemd-service
#Please enter Starlight Proxy address (example: proxy.mc256.dev:8090):172.18.1.3:8090
#Enable HTTPS Certificate (requires load balancer like Nginx) (y/N):n
#Created systemd service file (/lib/systemd/system/starlight.service)
#Reloaded systemd daemon
```

Enable Starlight snapshotter service
```shell
sudo systemctl enable starlight   && \
sudo systemctl start starlight
```

Verify Starlight is running
```shell
sudo systemctl status starlight
# it should be "active".
```

### 4. Configure `contaienrd`

Add configuration to `/etc/containerd/config.toml`. 
(If you have set other `proxy_plugins`, please manually edit the file)
```shell
sudo mkdir /etc/containerd/ && \
cat <<EOT | sudo tee -a /etc/containerd/config.toml > /dev/null
[proxy_plugins]
  [proxy_plugins.starlight]
    type = "snapshot"
    address = "/run/starlight-grpc/starlight-snapshotter.socket"
EOT
```

Restart `containerd` service
```shell
sudo systemctl restart containerd
```

Verify the Starlight snapshotter plugin is functioning
```shell
sudo ctr plugin ls | grep starlight 
# io.containerd.snapshotter.v1    starlight                -              ok
```


🙌 That's it. Please proceed to the **Step 3**.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started) 


---

For more information, please see `ctr-starlight --help` and `starlight-grpc --help`
