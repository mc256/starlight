#!/bin/bash

# This script requires installation of Docker 
# (perhaps running this on the registry server)
# TODO: change it to use the `ctr` command


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
  "cassandra:3.11.10"
  "cassandra:3.11.9"
  "redis:5.0"
  "redis:6.0"
  "redis:6.2"

  "ubuntu:focal-20210416"
  "ubuntu:focal-20210401"
  "alpine:3.13.5"
  "alpine:3.13.4"
  "busybox:1.32.1"
  "busybox:1.33.0"
  "busybox:1.32.1"
  "busybox:1.32.0"
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
  "mariadb:10.5.9"
  "mariadb:10.5.8"

  "cassandra:3.11.10"
  "cassandra:3.11.9"
  "redis:5.0"
  "redis:6.0"
  "redis:6.2"
  "redis:6.2.2"
  "redis:6.2.1"
  "postgres:13.2"
  "postgres:13.1"
  "mongo:4.0.24"
  "mongo:4.0.23"

  "python:3.9.4"
  "python:3.9.3"
  "node:16-alpine3.12"
  "node:16-alpine3.11"
  "openjdk:16.0.1-jdk"
  "openjdk:11.0.11-9-jdk"
  "golang:1.16.3"
  "golang:1.16.2"

  "rabbitmq:3.8.14"
  "rabbitmq:3.8.13"

  "wordpress:php7.4-fpm"
  "wordpress:php7.3-fpm"


  "nextcloud:21.0.1-apache"
  "nextcloud:20.0.9-apache"

  "ghost:4.3.3-alpine"
  "ghost:3.42.5-alpine"
  "phpmyadmin:5.1.0-fpm-alpine"
  "phpmyadmin:5.0.4-fpm-alpine"

  "httpd:2.4.46"
  "httpd:2.4.43"

  "nginx:1.19.10"
  "nginx:1.20.0"

  "flink:1.12.3-scala_2.12-java8"
  "flink:1.12.3-scala_2.11-java8"

  "eclipse-mosquitto:2.0.10-openssl"
  "eclipse-mosquitto:2.0.9-openssl"

  "registry:2.7.1"
  "registry:2.7.0"

  "memcached:1.6.9"
  "memcached:1.6.8"
)

for VAL in "${ImageList[@]}"; do
  docker pull "$VAL"
  docker image tag "$VAL" "localhost:5000/$VAL"
  docker push "localhost:5000/$VAL"
done

read -p "The following command will remove all the images in docker, if you want to keep, please click Ctrl+C. Otherwise, Press enter to continue"
docker rmi -f $(docker images -q)