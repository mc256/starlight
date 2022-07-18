#FROM amd64/ubuntu:22.04
FROM amd64/ubuntu:jammy-20220531

WORKDIR /go/src/app

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info
ENV PATH="/usr/local/go/bin:${PATH}" 
ENV EMAIL=webmaster@mc256.dev
ENV TZ="America/New_York"

RUN apt update -y && apt upgrade -y 
RUN DEBIAN_FRONTEND=noninteractive TZ=Etc/UTC apt-get -y install tzdata
RUN apt install -y ssh vim tmux zsh git curl dpkg-dev gpg build-essential debhelper
RUN wget https://go.dev/dl/go1.18.4.linux-amd64.tar.gz && rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.4.linux-amd64.tar.gz 
COPY . .
RUN make create-deb-package

CMD [ "/bin/bash" ]