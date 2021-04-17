######################################################################
# Build
######################################################################
TARGETS=starlight-proxy starlight-grpc ctr-starlight

.PHONY: build clean build-starlight-proxy build-starlight-grpc

######################################################################
# Build
######################################################################
build: build-starlight-proxy build-starlight-grpc

build-starlight-proxy:
	-mkdir ./out
	go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

build-starlight-grpc:
	-mkdir ./out
	go build -o ./out/starlight-grpc ./cmd/starlight-grpc/main.go


######################################################################
# Clean
######################################################################
clean:
	-rm -rf ./out/*

