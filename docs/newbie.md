# TL;DR All-in-one Quick Start Guide

To finish this guide, you will need TWO machines (or VMs) far away from each other. 
One acts as the Cloud, and the other acts as the Edge. You will need to identify the IP address of the Cloud server.

The following instructions have been tested using AWS EC2 t2.micro with Ubuntu 20.04 LTS.


## The "Cloud"

In this machine you will need to set up the Starlight Proxy and a standard container registry. 
If you are using AWS EC2, please add port 8090 and port 5000 to the Security Group whitelist when you create the VM.

0. Install [Docker](https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository) and [Docker Compose](https://docs.docker.com/compose/install/)  

If using Ubuntu 20.04 LTS, you could install Docker and Docker Compose using the following commands: 
```shell
sudo apt update && \
sudo apt upgrade -y && \
sudo apt install -y docker-compose && \
sudo usermod -aG docker $USER
```
After adding the current user to the `docker` group, you (may) **need to log out and log in** to take effect.
To confirm that Docker is working with correct permission, `docker ps` should not print any errors.
```shell
docker ps
# CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
```

1. Clone this project and launch the registry and proxy containers from `./demo/compose/registry+proxy`

```shell
git clone https://github.com/mc256/starlight.git && \
cd starlight/demo/compose/registry+proxy && \
docker-compose up -d
# Creating network "registryproxy_default" with the default driver
# Creating starlightproxy    ... done
# Creating starlightregistry ... done
```
The Starlight proxy writes image metadata to `./data_proxy` folder, and
the container registry saves container images to `./data_registry`


2. Verify the registry and proxy are running.
```shell
curl http://localhost:8090/
# Starlight Proxy OK!
curl http://localhost:5000/v2/
# {}
```

The Starlight proxy listens on port 8090. 
We could put a Nginx reverse proxy to handle SSL certificates or load balancing.
But for simplicity, this part is ignored in this example.
Please add port 8090 and 5000 to the firewall whitelist, the worker has to access these ports.

3. Upload a few container images to the registry for testing

```shell
docker pull redis:6.2.1 && \
docker pull redis:6.2.2 && \
docker tag redis:6.2.1 localhost:5000/redis:6.2.1 && \
docker tag redis:6.2.2 localhost:5000/redis:6.2.2 && \
docker push localhost:5000/redis:6.2.1 && \
docker push localhost:5000/redis:6.2.2
```

You could upload other container images to the registry if you like.

🙌 That's it. Please obtain the IP address of this machine and run the following commands on the Edge server.

```shell
# update the IP address keep this for future use. 
export STARLIGHT_PROXY=<ip address of your server>:8090
export REGISTRY=<ip address of your server>:5000
```


## The "Edge"

Please get another machine (or VM), you will need to set up a container worker with Starlight Snapshotter plugin.

### 1. Install Dependencies

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

Find out the IP address / DNS of the Starlight Proxy server and set these two environment variables (Don't Copy-Paste!)
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
Please follow the prompt, enter (need the IP Address of the first machine!)
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

### 5. Convert Container Image

Convert the container image to the **Starlight format** container image.
```shell
ctr-starlight convert \
    --insecure-source --insecure-destination \
    $REGISTRY/redis:6.2.1 $REGISTRY/redis:6.2.1-starlight && \
ctr-starlight convert \
    --insecure-source --insecure-destination \
    $REGISTRY/redis:6.2.2 $REGISTRY/redis:6.2.2-starlight
```

In addition, the proxy needs some metadata about the list of files in the container to compute the data for deployment.
   ```shell
   curl http://$STARLIGHT_PROXY/prepare/redis:6.2.1-starlight
   #Cached TOC: redis:6.2.1-starlight
   curl http://$STARLIGHT_PROXY/prepare/redis:6.2.2-starlight
   #Cached TOC: redis:6.2.2-starlight
   ```



### 6. Optimize Container Image

Collect traces on the worker for container startup.
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

Repeat the same thing for `redis:6.2.2`
```shell
sudo ctr-starlight pull redis:6.2.2-starlight && \
sudo ctr-starlight create --optimize \
      --mount type=bind,src=/tmp/test-redis-data,dst=/data,options=rbind:rw \
   --env-file ./demo/config/all.env \
   --net-host \
   redis:6.2.2-starlight \
   redis:6.2.2-starlight \
   instance2 && \
sudo ctr task start instance2
```

Terminate the container using `Ctrl-C`, and remove the container:
```shell
sudo ctr container rm instance2
```

Report traces to the Starlight Proxy.
```shell
ctr-starlight report --server $STARLIGHT_PROXY --plain-http
```


### 7. Clear all the cache and reset the environment
```shell
sudo ./demo/reset.sh
```


### 8. Deploying and update container

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

Update a container using Starlight
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
