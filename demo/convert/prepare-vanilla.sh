#!/bin/bash

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
  "mysql:8.0.20"
  "mysql:8.0.21"
  "mysql:8.0.22"
	"mysql:8.0.23"
	"mysql:8.0.24"
	"mariadb:10.4"
	"mariadb:10.5"
  "cassandra:3.11"
  "cassandra:4.0"
  "redis:5.0"
  "redis:6.0"
  "redis:6.2"
)

for VAL in "${ImageList[@]}"; do
  docker pull "$VAL"
  docker image tag "$VAL" "localhost:5000/$VAL"
  docker push "localhost:5000/$VAL"
done