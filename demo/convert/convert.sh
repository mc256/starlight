#!/bin/bash

REGISTRY="cloudy:5000"
echo $REGISTRY

declare -a ImageList=(
	"ubuntu:18.04"
	"ubuntu:20.04"
	"alpine:3.12.7"
	"alpine:3.13.5"
	"busybox:1.32.1"
	"busybox:1.33.0"
	"debian:oldstable"
	"debian:stable"
	"centos:7"
	"centos:8"
	"fedora:32"
	"fedora:33"
	"oraclelinux:7"
	"oraclelinux:8"
)

for VAL in "${ImageList[@]}"; do
  echo "============================================================"
  echo $VAL
  echo "============================================================"
  ctr-remote image optimize --plain-http --entrypoint='[ "/bin/sh", "-c" ]' --args='[ "echo hello" ]' \
  	"$VAL" "http://$REGISTRY/$VAL-starlight"
done

declare -a DatabaseList=(
  "mysql:8.0.20"
  "mysql:8.0.21"
  "mysql:8.0.22"
	"mysql:8.0.23"
	"mysql:8.0.24"
	"mariadb:10.4"
	"mariadb:10.5"
)

for VAL in "${DatabaseList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  mkdir /tmp/t2
  chown -R 999:999 /tmp/t1
  chown -R 999:999 /tmp/t2

  ctr-remote image optimize --plain-http \
    --env-file=../config/all.env \
    --mount type=bind,src=/tmp/t1,dst=/var/lib/mysql,options=bind:rw \
  	--mount type=bind,src=/tmp/t2,dst=/run/mysqld,options=bind:rw \
	  --wait-on-signal \
	"$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
	rm -rf /tmp/t2
done


declare -a CassandraList=(
  "cassandra:3.11"
  "cassandra:4.0"
)

for VAL in "${CassandraList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"
done


declare -a RedisList=(
  "redis:5.0"
  "redis:6.0"
  "redis:6.2"
)
for VAL in "${RedisList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/data,options=bind:rw \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
done
