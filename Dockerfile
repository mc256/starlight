ARG CONTAINERD_VERSION=v1.5.0-rc.0
ARG RUNC_VERSION=v1.0.0-rc93

#############
# Proxy
#############
FROM golang:1.15 AS starlight-proxy

WORKDIR /go/src/app
COPY . .

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info

RUN make build-starlight-proxy
EXPOSE 8090

CMD ["sh", "-c", "/go/src/app/out/starlight-proxy $REGISTRY $LOGLEVEL"]



#############
# CLient
#############

######
FROM golang:1.15-buster AS containerd-base

ARG CONTAINERD_VERSION
ARG RUNC_VERSION

WORKDIR /go/src/app

RUN apt-get update -y && apt-get upgrade -y && \
    apt-get install -y libbtrfs-dev libseccomp-dev btrfs-progs libseccomp2

RUN git clone -b ${CONTAINERD_VERSION} --depth 1 \
    https://github.com/containerd/containerd /go/src/github.com/containerd/containerd && \
    git clone -b ${RUNC_VERSION} --depth 1 \
    https://github.com/opencontainers/runc /go/src/github.com/opencontainers/runc

RUN cd /go/src/github.com/containerd/containerd && \
    GO111MODULE=off make && make install && \
    cd /go/src/github.com/opencontainers/runc && \
    GO111MODULE=off make && make install

ENTRYPOINT ["bash"]

######
FROM containerd-base as starlight-worker

ENV PATH="/go/src/app/starlight/out:$PATH"

WORKDIR /go/src/app/starlight

COPY . .
COPY ./demo/config/containerd.config.toml /etc/containerd/config.toml
COPY ./demo/config/starlight-snapshotter-entrypoint.sh /entrypoint.sh

RUN make build-starlight-grpc
RUN make build-ctr-starlight

ENTRYPOINT ["/starlight-snapshotter-entrypoint.sh"]

######
FROM starlight-worker AS experiment-base

ENV VIRTUAL_ENV=/opt/venv

ENV TERM=xterm
ENV LC_ALL="en_US.UTF-8"
ENV LC_TYPE="en_US"

WORKDIR /go/src/app/starlight/demo

RUN apt install -y zsh python3 python3-pip python3-venv curl wget iproute2 htop vim git tmux iperf iftop ncdu autossh ssh && \
    wget https://github.com/robbyrussell/oh-my-zsh/raw/master/tools/install.sh -O - | zsh || true
RUN chsh -s /bin/zsh

RUN python3 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"
RUN pip install pandas matplotlib numpy jupyter

COPY ./demo/config/jupyer_notebook_config.py /root/.jupyter/jupyer_notebook_config.py

VOLUME /root/.ssh
VOLUME /root/.jupyter

ENTRYPOINT ["zsh"]


