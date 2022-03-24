# Starlight Proxy


This is the **step 1** to use Starlight:

Set up a Starlight proxy, ideally close to the registry server you are using. 
Configure the proxy server to point to the registry and run it. Starlight supports any standard registry.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started)

---
## Method 1. Use Docker Compose to deploy Starlight Proxy + Container Registry (Recommended)

This is an all-in-one example in case you don't have full access to a container registry.
We could use Docker Compose to deploy both the proxy and the registry on the same machine.


0. Install [Docker](https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository) and [Docker Compose](https://docs.docker.com/compose/install/)  

If using Ubuntu 20.04 LTS, you could install Docker and Docker Compose using the following commands: 
```shell
sudo apt install docker-compose && \
sudo usermod -aG docker $USER
```
After adding the current user to the `docker` group, you may need to log out and log in to take effect.
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

3. Upload a few container images to the registry for testing

```shell
docker pull redis:6.2.1 && \
docker pull redis:6.2.2 && \
docker tag redis:6.2.1 localhost:5000/redis:6.2.1 && \
docker tag redis:6.2.2 localhost:5000/redis:6.2.2 && \
docker push localhost:5000/redis:6.2.1 && \
docker push localhost:5000/redis:6.2.2
```
This step is optional, you could upload other container images to the registry.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started) 

---
## Method 2. Use Docker Compose (Starlight Only)

The prebuilt Starlight proxy container image is available at  `ghcr.io/mc256/starlight/proxy:latest`.

0. Install [Docker](https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository) and [Docker Compose](https://docs.docker.com/compose/install/)  

If using Ubuntu 20.04 LTS, you could install Docker and Docker Compose using the following commands: 
```shell
sudo apt install docker-compose && \
sudo usermod -aG docker $USER
```
After adding the current user to the `docker` group, you may need to log out and log in to take effect.
To confirm that Docker is working with correct permission, `docker ps` should not print any errors.
```shell
docker ps
# CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES
```

1. Clone this project 

```shell
git clone https://github.com/mc256/starlight.git && \
```

2. Set `REGISTRY` environment variable to your own container registry. 

```shell
echo "REGISTRY=http://starlightregistry:5000" >> ./starlight/demo/compose/proxy/.env
```

3. Launch the proxy
```shell
cd ./starlight/demo/compose/proxy && \
docker-compose up -d
# Creating starlightproxy ... done
```

The Starlight proxy writes image metadata to `./data_proxy` folder.

2. Verify the registry and proxy are running.
```shell
curl http://localhost:8090/
# Starlight Proxy OK!
```

The Starlight proxy listens on port 8090. 
We could put a Nginx reverse proxy to handle SSL certificates or load balancing.
But for simplicity, this part is ignored in this example.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started)

---
## Method 3. Build from source

0. Install Go https://go.dev/doc/install ➡️
```shell
wget https://go.dev/dl/go1.17.8.linux-amd64.tar.gz &&
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
```

1. Add Go to the environment variable (You may want to change `.zshrc` or `.bashrc` file to permanently add this folder to the `PATH` environment variable)
```
export PATH=$PATH:/usr/local/go/bin
```

2. Verify Go is available with `go version`
```shell
go version
#go version go1.17.8 linux/amd64
```

4. Install necessary tools to build this project

```shell
sudo apt update && \
sudo apt upgrade -y && \
sudo apt install build-essential
```

4. Clone this project.

```shell
git clone https://github.com/mc256/starlight.git && \
cd starlight
```

5. Build Starlight proxy
```shell
make build-starlight-proxy
```

6. Run Starlight
```shell
cd ./out && \
mkdir ./data && \
./starlight-proxy http://myregistry:5000 &
```

7. Verify the Starlight Proxy is working
```shell
curl http://localhost:8090/
# Starlight Proxy OK!
```

The Starlight proxy listens on port 8090. 
We could put a Nginx reverse proxy to handle SSL certificates or load balancing.
But for simplicity, this part is ignored in this example.

[⬅️ Back to README.md](https://github.com/mc256/starlight#getting-started)

---
## Known Issues

1) Authentication is not supported yet. But will be implemented very soon.
2) We should switch to `nerdctl` ASAP.