FROM golang:1.18 AS starlight-proxy-build

WORKDIR /go/src/app
COPY . .

ENV GO111MODULE=on

RUN make change-version-number set-production starlight-proxy-for-alpine && mkdir ./out/data

#CMD ["/go/src/app/out/starlight-proxy"]
FROM alpine:3.12 AS starlight-proxy

COPY --from=starlight-proxy-build /go/src/app/out/ /opt/
WORKDIR /opt
EXPOSE 8090
CMD ["/opt/starlight-proxy"]

FROM golang:1.18 AS starlight-cli-build

WORKDIR /go/src/app
COPY . .

RUN make change-version-number set-production ctr-starlight-for-alpine && mkdir ./out/data

FROM alpine:3.12 AS starlight-cli

COPY --from=starlight-cli-build /go/src/app/out/ /opt/
WORKDIR /opt
EXPOSE 8090
CMD ["/opt/ctr-starlight"]