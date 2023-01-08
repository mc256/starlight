######################################################################
# Build
######################################################################
TARGETS=starlight-proxy starlight-daemon ctr-starlight
COMMONENVVAR=GOOS=$(shell uname -s | tr A-Z a-z)
BUILDENVVAR=CGO_ENABLED=0
VERSION=$(shell git describe --tags --match "v*" || echo "v0.0.0")
VERSIONNUMBER=$(shell echo $(VERSION) | sed 's/v//g')
COMPILEDATE=$(shell date +%Y%m%d)


.PHONY: build clean starlight-proxy starlight-daemon ctr-starlight
.SILENT: install-systemd-service

######################################################################
# Build
######################################################################
.PHONY: build
build: starlight-proxy starlight-daemon ctr-starlight

.PHONY: starlight-proxy
starlight-proxy:
	-mkdir ./out 2>/dev/null | true
	go mod tidy
	go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

.PHONY: starlight-daemon
starlight-daemon:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/starlight-daemon ./cmd/starlight-daemon/main.go

.PHONY: ctr-starlight
ctr-starlight:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/ctr-starlight ./cmd/ctr-starlight/main.go

.PHONY: ctr-starlight-for-alpine
ctr-starlight-for-alpine:
	-mkdir ./out 2>/dev/null | true
	go mod vendor
	go mod tidy
	$(COMMONENVVAR) $(BUILDENVVAR)	go build -o ./out/ctr-starlight ./cmd/ctr-starlight/main.go

.PHONY: starlight-proxy-for-alpine
starlight-proxy-for-alpine:
	-mkdir ./out 2>/dev/null | true
	go mod vendor
	go mod tidy
	$(COMMONENVVAR) $(BUILDENVVAR) go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

.PHONY: helm-package
helm-package:
	helm package ./demo/chart --version $(VERSIONNUMBER) -d /tmp

.PHONY: push-helm-package
push-helm-package:
	helm push /tmp/starlight-$(VERSIONNUMBER).tgz oci://ghcr.io/mc256/starlight/

.PHONY: change-version-number
change-version-number:
	sed -i 's/var Version = "0.0.0"/var Version = "$(VERSIONNUMBER)-$(COMPILEDATE)"/g' ./util/version.go

.PHONY: set-production
set-production:
	sed -i 's/production = false/production = true/g' ./util/config.go

.PHONY: generate-changelog
generate-changelog: 
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/ 2>/dev/null | true
	sh -c ./demo/deb-package/generate-changelog.sh > ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/changelog

.PHONY: create-deb-package
create-deb-package: change-version-number set-production starlight-daemon ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight_$(VERSIONNUMBER)_amd64.deb

.PHONY: update-protobuf
update-protobuf:
	protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    client/api/daemon.proto


######################################################################
###### Platform dependent build

.PHONY: create-deb-package.amd64
create-deb-package.amd64: create-deb-package


.PHONY: create-deb-package.armv6l
create-deb-package.armv6l: change-version-number set-production starlight-daemon ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	sed -i 's/Architecture: amd64/Architecture: armhf/g' ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight_$(VERSIONNUMBER)_armhf.deb

.PHONY: create-deb-package.arm64
create-deb-package.arm64: change-version-number set-production starlight-daemon ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	sed -i 's/Architecture: amd64/Architecture: arm64/g' ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight_$(VERSIONNUMBER)_arm64.deb


.PHONY: upload-deb-package.amd64
upload-deb-package.amd64:
	curl --form uploadfile='@./sandbox/starlight_$(VERSIONNUMBER)_amd64.deb' $(UPLOAD_URL)
	#curl -X POST -u $(APT_UPLOAD_AUTH) -F starlight_$(VERSIONNUMBER)_amd64.deb='@./sandbox/starlight_$(VERSIONNUMBER)_amd64.deb' https://repo.yuri.moe/api/files/starlight

.PHONY: upload-deb-package.armv6l
upload-deb-package.armv6l:
	curl --form uploadfile='@./sandbox/starlight_$(VERSIONNUMBER)_armhf.deb' $(UPLOAD_URL)
	#curl -X POST -u $(APT_UPLOAD_AUTH) -F starlight_$(VERSIONNUMBER)_armhf.deb='@./sandbox/starlight_$(VERSIONNUMBER)_armhf.deb' https://repo.yuri.moe/api/files/starlight

.PHONY: upload-deb-package.amd64
upload-deb-package.arm64:
	curl --form uploadfile='@./sandbox/starlight_$(VERSIONNUMBER)_arm64.deb' $(UPLOAD_URL)
	#curl -X POST -u $(APT_UPLOAD_AUTH) -F starlight_$(VERSIONNUMBER)_arm64.deb='@./sandbox/starlight_$(VERSIONNUMBER)_arm64.deb' https://repo.yuri.moe/api/files/starlight

######################################################################
.PHONY: docker-buildx-multi-arch
docker-buildx-multi-arch:
	docker buildx build \
        --platform linux/amd64 \
		--build-arg ARCH=amd64 \
		--build-arg UPLOAD_URL=$(UPLOAD_URL) \
 		--build-arg APT_UPLOAD_AUTH=$(APT_UPLOAD_AUTH)  \
 		--network=host \
 		-f ./demo/deb-package/Dockerfile .
	docker buildx build \
        --platform linux/arm/v7 \
		--build-arg ARCH=armv6l \
		--build-arg UPLOAD_URL=$(UPLOAD_URL) \
 		--build-arg APT_UPLOAD_AUTH=$(APT_UPLOAD_AUTH)  \
 		--network=host \
 		-f ./demo/deb-package/Dockerfile .
	docker buildx build \
        --platform linux/arm64/v8 \
		--build-arg ARCH=arm64 \
		--build-arg UPLOAD_URL=$(UPLOAD_URL) \
 		--build-arg APT_UPLOAD_AUTH=$(APT_UPLOAD_AUTH)  \
 		--network=host \
 		-f ./demo/deb-package/Dockerfile .

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
	cp ./demo/deb-package/debian/starlight.service /lib/systemd/system/
	systemctl daemon-reload

docker-image:
	docker build -t harbor.yuri.moe/public/starlight-proxy:latest -f Dockerfile --target starlight-proxy .
	docker build -t harbor.yuri.moe/public/starlight-cli:latest -f Dockerfile --target starlight-cli .