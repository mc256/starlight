#!/bin/bash

#SERVER="container-worker.momoko:8090"
#SERVER="starlight:8090"
SERVER="proxy.starlight.yuri.moe"

declare -a ImageList=(

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

)

for VAL in "${ImageList[@]}"; do
  echo "https://$SERVER/prepare/$VAL-starlight"
  curl "https://$SERVER/prepare/$VAL-starlight"
done
