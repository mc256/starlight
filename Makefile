TARGETS=starlight-proxy starlight-grpc ctr-starlight

.PHONY: build clean


######################################################################
# Build
######################################################################
build: starlight-proxy starlight-grpc

starlight-proxy:
	go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

starlight-grpc:
	go build -o ./out/starlight-grpc ./cmd/starlight-grpc/main.go


######################################################################
# Clean
######################################################################
clean:
	rm -rf ./out/*

