FROM golang:1.15

WORKDIR /go/src/app
COPY . .

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info

RUN make build-starlight-proxy
EXPOSE 8090

CMD ["sh", "-c", "/go/src/app/out/stargz-proxy $REGISTRY $LOGLEVEL"]