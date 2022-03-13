# Starlight Proxy


This is the **step 1** to use Starlight:

Set up a Starlight proxy, ideally close to the registry server you are using. 
Configure the proxy server to point to the registry and run it. Starlight supports any standard registry.

[⬅️ Back to README.md](https://github.com/mc256/starlight)



---

## Method 1. Use Docker Compose (Recommended)

The prebuilt Starlight proxy container image is available at  `ghcr.io/mc256/starlight/proxy:latest`.
We could use Docker Compose to deploy both the proxy and the registry on the same machine. 

0. Install Docker and Docker Compose  

Install [Docker ➡️](https://docs.docker.com/engine/install/ubuntu/#install-using-the-repository)

Install [Docker Compose ➡️](https://docs.docker.com/compose/install/) 

To confirm that Docker is working with correct permission, `docker ps` should not print any errors.

1. Create `docker-compose.yml` file in an empty folder

```yaml
version: "3"
services:
  starlightproxy:
    image: ghcr.io/mc256/starlight/proxy:latest
    container_name: starlightproxy
    ports:
      - 8090:8090
    env_file:
      - config.env
    volumes:
      - "./data:/go/src/app/data:rw"
    restart: always
```

The Starlight proxy listens on port 8090. 
We should put a Nginx reverse proxy to handle SSL certificates or load balancing.
But for simplicity, this part is ignored in this example.
The Starlight proxy writes image metadata to `./data` folder, 
and it needs some environment variables in `config.env` (details are in the next step).


2. Create `config.env` file in the same folder. This configuration points the proxy to the registry.
(You may want to change `starlightregistry` to your container registry.)
```dotenv
REGISTRY=https://starlightregistry
LOGLEVEL=info
```
 

3. Launch the container 
```shell
docker-compose up -d
```

Deployments with registry, Nginx reverse proxy and other examples are available in [`demo/compose`](https://github.com/mc256/starlight/tree/master/demo/compose) folder in this repository.


## Method 2. Build from source


0. Install Go https://go.dev/doc/install ➡️

```shell
wget https://go.dev/dl/go1.17.8.linux-amd64.tar.gz &&
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.17.8.linux-amd64.tar.gz
```

1. Add Go to the environment variable (You may want to change `.zshrc` or `.bashrc` file to permanently add this folder to the `PATH` environment variable)

```
export PATH=$PATH:/usr/local/go/bin
```

3. Install necessary tools to build this project

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

6. Verify the Starlight Proxy is working
```shell
curl http://localhost:8090/
```

---
## Known Issues

1) Authentication is not supported yet. But will be implemented very soon.
2) We should switch to `nerdctl` ASAP.