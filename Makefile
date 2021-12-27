######################################################################
# Build
######################################################################
TARGETS=starlight-proxy starlight-grpc ctr-starlight

.PHONY: build clean build-starlight-proxy build-starlight-grpc build-ctr-starlight install

######################################################################
# Build
######################################################################
build: build-starlight-proxy build-starlight-grpc build-ctr-starlight

build-starlight-proxy:
	-mkdir ./_out 2>/dev/null | true
	go build -o ./_out/starlight-proxy ./cmd/starlight-proxy/main.go

build-starlight-grpc:
	-mkdir ./_out 2>/dev/null | true
	go build -o ./_out/starlight-grpc ./cmd/starlight-grpc/main.go

build-ctr-starlight:
	-mkdir ./_out 2>/dev/null | true
	go build -o ./_out/ctr-starlight ./cmd/ctr-starlight/main.go

######################################################################
# Clean
######################################################################
clean:
	-rm -rf ./_out/*


######################################################################
# Install
######################################################################
install:
	install ./_out/*-* /bin
