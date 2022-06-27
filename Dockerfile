ARG CONTAINERD_VERSION=v1.5.0-rc.0
ARG RUNC_VERSION=v1.0.0-rc93

#############
# Proxy
#############
FROM golang:1.18 AS starlight-proxy

WORKDIR /go/src/app
COPY . .

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info

RUN make build-starlight-proxy
EXPOSE 8090

#############
FROM alpine:3.12

COPY --from=0 /go/src/app/out/starlight-proxy /bin/starlight-proxy

WORKDIR /bin
CMD ["sh", "-c", "/bin/starlight-proxy $REGISTRY $LOGLEVEL"]

