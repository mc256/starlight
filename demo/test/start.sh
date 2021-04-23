#!/bin/bash

STARGZ_SNAPSHOTTER_ROOT=/var/lib/containerd-stargz-grpc/
STARLIGHT_SNAPSHOTTER_ROOT=/var/lib/starlight-grpc/

#systemctl stop containerd
ps aux | grep /usr/local/bin/containerd | awk '{print $2}' | sudo xargs kill -9

rm -rf /var/lib/containerd

if [ -d "${STARGZ_SNAPSHOTTER_ROOT}snapshotter/snapshots/" ] ; then
    find "${STARGZ_SNAPSHOTTER_ROOT}snapshotter/snapshots/" \
         -maxdepth 1 -mindepth 1 -type d -exec umount "{}/fs" \;
fi
rm -rf "${STARGZ_SNAPSHOTTER_ROOT}"*

if [ -d "${STARLIGHT_SNAPSHOTTER_ROOT}sfs/" ] ; then
    find "${STARLIGHT_SNAPSHOTTER_ROOT}sfs/" \
         -maxdepth 1 -mindepth 1 -type d -exec umount "{}/m" \;
fi
rm -rf "${STARLIGHT_SNAPSHOTTER_ROOT}"*

ps aux | grep ctr-starlight | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep ctr-remote | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep " starlight-grpc " | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep " stargz-grpc " | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep shim-runc-v2 | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep entrypoint | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep mysqld | head -n -1 | awk '{print $2}' | sudo xargs kill -9
ps aux | grep redis-server | head -n -1 | awk '{print $2}' | sudo xargs kill -9

/usr/local/bin/containerd &