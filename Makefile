######################################################################
# Build
######################################################################
TARGETS=starlight-proxy starlight-grpc ctr-starlight

.PHONY: build clean build-starlight-proxy build-starlight-grpc build-ctr-starlight install install-systemd-service
.SILENT: install-systemd-service

######################################################################
# Build
######################################################################
build: build-starlight-proxy build-starlight-grpc build-ctr-starlight

build-starlight-proxy:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

build-starlight-grpc:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/starlight-grpc ./cmd/starlight-grpc/main.go

build-ctr-starlight:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/ctr-starlight ./cmd/ctr-starlight/main.go

######################################################################
# Clean
######################################################################
clean:
	-rm -rf ./out/*


######################################################################
# Install
######################################################################
install:
	install ./out/*-* /usr/bin

install-systemd-service:
	./demo/install.sh
	#@printf "Please enter Starlight Proxy address (example: \033[92mproxy.mc256.dev:8090\033[0m):"
	#@read proxy_address; \
	#echo $$proxy_address; \
	#service_file=`cat './demo/starlight.service'`; \
	#echo `subst "STARLIGHT_PROXY",$(proxy_address),$(service_file)`; \
	#cp ./demo/starlight.service /lib/systemd/system/
	#systemctl daemon-reload