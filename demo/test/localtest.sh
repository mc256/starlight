#!/bin/bash

# You should run this script with sudo and in the same folder
if [ -z "$1" ]
  then
    echo "Usage: $0 ProjectPath "
    exit 0
fi
cd "$1" || exit

SERVER=http://10.219.31.127:5000

out/starlight-proxy $SERVER &
out/starlight-grpc run --log-level=trace \
 --plain-http \
 --fs=/var/lib/starlight-grpc/ \
 --socket=/run/starlight-grpc/starlight-snapshotter.socket \
 --server=localhost:8090 &

out/ctr-starlight prepare ubuntu:18.04-starlight

out/ctr-starlight create --tty --net-host --local-time \
  ubuntu:18.04-starlight ubuntu:18.04-starlight \
  task1 /bin/bash
ctr t start task1