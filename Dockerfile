FROM amd64/golang:1.18 AS starlight-proxy

WORKDIR /go/src/app
COPY . .

ENV GO111MODULE=on
ENV REGISTRY=registry2
ENV LOGLEVEL=info

RUN make build-starlight-proxy-for-alpine && mkdir ./out/data

#CMD ["/go/src/app/out/starlight-proxy"]
FROM amd64/alpine:3.12

COPY --from=0 /go/src/app/out/ /opt/
WORKDIR /opt
EXPOSE 8090
CMD ["/opt/starlight-proxy"]