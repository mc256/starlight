FROM debian:stable

WORKDIR /go/src/app

ARG ARCH=amd64
ARG UPLOAD_URL=http://127.0.0.1:35001/
ARG APT_UPLOAD_AUTH=XXXX

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info
ENV PATH="/usr/local/go/bin:${PATH}" 
ENV EMAIL=webmaster@mc256.dev
ENV DEBIAN_FRONTEND=noninteractive 
ENV TZ=Etc/UTC

RUN apt update -y && apt upgrade -y 
RUN apt install -y tzdata curl wget git dpkg-dev gpg build-essential debhelper bind9-utils
RUN wget -q https://go.dev/dl/go1.20.8.linux-$ARCH.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.20.8.linux-$ARCH.tar.gz 
COPY . .
RUN ARCH=$ARCH && make create-deb-package.$ARCH upload-deb-package.$ARCH
