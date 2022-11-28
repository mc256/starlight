#!/usr/bin/env bash

# remove containerd filesystems
rm -rf /var/lib/rancher/k3s/agent/containerd

# remove starlight layer cache
rm -rf /var/lib/starlight/layers


# systemctl restart k3s-agent
# systemctl stop containerd