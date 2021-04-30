class Configuration:
    REGISTRY_SERVER = "cloudy:5000"
    PROXY_SERVER = "starlight:8090"
    #REGISTRY_SERVER = "container-worker.momoko:5000"
    #PROXY_SERVER = "container-worker.momoko:8090"

    NETWORK_DEVICE_WORKER = "enp3s0"
    NETWORK_DEVICE_REGISTRY = "enp3s0"
    NETWORK_DEVICE_STARLIGHT_PROXY = "enp4s0"
    #NETWORK_DEVICE_WORKER = "mpqemubr0"
    #NETWORK_DEVICE_REGISTRY = "ens4"
    #NETWORK_DEVICE_STARLIGHT_PROXY = "ens4"

    SSH_TO_REGISTRY = ["ssh", "maverick@cloudy"]
    SSH_TO_STARLIGHT_PROXY = ["ssh", "maverick@starlight"]
    #SSH_TO_REGISTRY = ["multipass", "exec", "docker", "--"]
    #SSH_TO_STARLIGHT_PROXY = ["multipass", "exec", "docker", "--"]

    STARGZ_GRPC = "stargz-grpc"
    STARLIGHT_GRPC = "starlight-grpc"

    TEE_LOG_CONTAINERD = " | tee /tmp/containerd.log"
    TEE_LOG_STARLIGHT = " | tee /tmp/starlight-grpc.log"
    TEE_LOG_STARGZ = " | tee /tmp/stargz-grpc.log"

    TEE_LOG_CONTAINERD_RUNTIME = " | tee -a /tmp/containerd-runtime.log"
    TEE_LOG_STARLIGHT_RUNTIME = " | tee -a /tmp/starlight-runtime.log"
    TEE_LOG_STARGZ_RUNTIME = " | tee -a /tmp/stargz-runtime.log"

    TMP = "/tmp"
    ENV = "../config/all.env"
