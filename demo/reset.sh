#!/bin/bash

STARLIGHT_SNAPSHOTTER_ROOT=/var/lib/starlight-grpc/

systemctl stop containerd
systemctl stop starlight
sudo pkill -9 'containerd' | true
sudo pkill -9 'starlight-grpc' | true

rm -rf /var/lib/containerd

if [ -d "${STARLIGHT_SNAPSHOTTER_ROOT}sfs/" ] ; then
    find "${STARLIGHT_SNAPSHOTTER_ROOT}sfs/" \
         -maxdepth 1 -mindepth 1 -type d -exec sudo umount -f "{}/m" \;
fi
rm -rf "${STARLIGHT_SNAPSHOTTER_ROOT}"*

rm -rf /tmp/test-redis-data

systemctl start starlight
systemctl start containerd
