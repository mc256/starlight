# Starlight Snapshotter Plugin and CLI tool

This is the **step 2** to use Starlight:

Set up the worker to be able to run Starlight. 
This involves 
installing **containerd** and the **Starlight snapshotter plugin**, 
configuring containerd to use the plugin, 
and starting the Starlight snapshotter daemon
(you also need to tell the snapshotter the address of the proxy server).
<br>[Find out how to install **containerd** ➡️](https://containerd.io/downloads/)


[⬅️ Back to README.md](https://github.com/mc256/starlight)

---

### Step 1. Install Dependencies

This uses Ubuntu 20.04 as an example. We will need `build-essential` and `containerd`.

```shell
sudo apt update && sudo apt upgrade -y && \
sudo apt install build-essential containerd
```

Enable `containerd`
```shell
sudo systemctl enable containerd
sudo systemctl start containerd
```

Verify `containerd` is running
```shell
sudo systemctl status containerd
```

Install Go https://go.dev/doc/install ➡️

```shell
wget https://go.dev/dl/go1.17.8.linux-amd64.tar.gz &&
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
```

Add Go to the environment variable (You may want to change `.zshrc` or `.bashrc` file to permanently add this folder to the `PATH` environment variable)

```shell
export PATH=$PATH:/usr/local/go/bin
```

Verify Go is available
```shell
go version
```


### Step 2. Clone and Build

Clone the Starlight repository
```shell
git clone https://github.com/mc256/starlight.git &&
cd starlight
```


Build the snapshotter plugin and CLI tool
```shell
make build-starlight-grpc build-ctr-starlight
```

Install snapshotter plugin and CLI tool
```shell
sudo make install install-systemd-service
```

Enable Starlight snapshotter service
```shell
sudo systemctl enable starlight
sudo systemctl start starlight
```

Verify Starlight is running
```shell
sudo systemctl status starlight
# it should be "active".
```

### Step 3. Configure Snapshotter

Add the following configuration to `/etc/containerd/config.toml`.
```toml
[proxy_plugins]
  [proxy_plugins.starlight]
    type = "snapshot"
    address = "/run/starlight-grpc/starlight-snapshotter.socket"
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

