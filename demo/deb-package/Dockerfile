#FROM amd64/ubuntu:22.04
FROM ubuntu:22.04

WORKDIR /go/src/app

ARG ARCH=amd64
ARG UPLOAD_URL=http://127.0.0.1:35001/

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info
ENV PATH="/usr/local/go/bin:${PATH}" 
ENV EMAIL=webmaster@mc256.dev
ENV DEBIAN_FRONTEND=noninteractive 
ENV TZ=Etc/UTC

RUN apt update -y && apt upgrade -y 
RUN apt install -y tzdata curl wget git dpkg-dev gpg build-essential debhelper
RUN wget -q https://go.dev/dl/go1.18.4.linux-$ARCH.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.4.linux-$ARCH.tar.gz 
COPY . .
RUN ARCH=$ARCH && make create-deb-package.$ARCH upload-deb-package.$ARCH
