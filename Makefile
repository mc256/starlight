######################################################################
# Build
######################################################################
TARGETS=starlight-proxy starlight-grpc ctr-starlight
COMMONENVVAR=GOOS=$(shell uname -s | tr A-Z a-z)
BUILDENVVAR=CGO_ENABLED=0
VERSION=$(shell git describe --tags --match "v*" || echo "v0.0.0")
VERSIONNUMBER=$(shell echo $(VERSION) | sed 's/v//g')
COMPILEDATE=$(shell date +%Y%m%d)


.PHONY: build clean build-starlight-proxy build-starlight-grpc build-ctr-starlight

.SILENT: install-systemd-service

######################################################################
# Build
######################################################################
.PHONY: build
build: build-starlight-proxy build-starlight-grpc build-ctr-starlight

.PHONY: build-starlight-proxy
build-starlight-proxy:
	-mkdir ./out 2>/dev/null | true
	go mod tidy
	go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

.PHONY: build-starlight-grpc
build-starlight-grpc:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/starlight-grpc ./cmd/starlight-grpc/main.go

.PHONY: build-ctr-starlight
build-ctr-starlight:
	-mkdir ./out 2>/dev/null | true
	go build -o ./out/ctr-starlight ./cmd/ctr-starlight/main.go


.PHONY: build-starlight-proxy-for-alpine
build-starlight-proxy-for-alpine:
	-mkdir ./out 2>/dev/null | true
	go mod vendor
	go mod tidy
	$(COMMONENVVAR) $(BUILDENVVAR) go build -o ./out/starlight-proxy ./cmd/starlight-proxy/main.go

.PHONY: build-helm-package
build-helm-package:
	helm package ./demo/chart --version $(VERSIONNUMBER) -d /tmp

.PHONY: push-helm-package
push-helm-package:
	helm push /tmp/starlight-proxy-chart-$(VERSIONNUMBER).tgz oci://ghcr.io/mc256/starlight/

.PHONY: change-version-number
change-version-number:
	sed -i 's/var Version = "0.0.0"/var Version = "$(VERSIONNUMBER)-$(COMPILEDATE)"/g' ./util/version.go

.PHONY: set-production
set-production:
	sed -i 's/production = false/production = true/g' ./util/config.go

.PHONY: generate-changelog
generate-changelog: 
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/ 2>/dev/null | true
	sh -c ./demo/deb-package/generate-changelog.sh > ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/changelog

.PHONY: create-deb-package
create-deb-package: change-version-number set-production build-starlight-grpc build-ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_amd64.deb

######################################################################
###### Platform dependent build

.PHONY: create-deb-package.amd64
create-deb-package.amd64: create-deb-package

.PHONY: upload-deb-package.amd64
upload-deb-package.amd64:
	curl --form uploadfile='@./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_amd64.deb' $(UPLOAD_URL)

.PHONY: create-deb-package.armv6l
create-deb-package.armv6l: change-version-number set-production build-starlight-grpc build-ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	sed -i 's/Architecture: amd64/Architecture: armhf/g' ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_armhf.deb

.PHONY: upload-deb-package.armv6l
upload-deb-package.armv6l:
	curl --form uploadfile='@./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_armhf.deb' $(UPLOAD_URL)

.PHONY: create-deb-package.arm64
create-deb-package.arm64: change-version-number set-production build-starlight-grpc build-ctr-starlight generate-changelog
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/ 2>/dev/null | true
	cp -r ./demo/deb-package/debian ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/
	mkdir -p ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/ 2>/dev/null | true
	cp -r ./out/* ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/starlight-snapshotter/usr/bin/
	sed -i 's/Standards-Version: 0.0.0/Standards-Version: $(VERSIONNUMBER)/g' ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	sed -i 's/Architecture: amd64/Architecture: arm64/g' ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE)/debian/control
	cd ./sandbox/starlight-snapshotter-$(VERSIONNUMBER)-$(COMPILEDATE) ; \
	dh_systemd_enable; \
	dh_systemd_start; \
	dh_installdeb; \
	dh_gencontrol; \
	dh_md5sums; \
	dh_builddeb
	dpkg-deb --info ./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_arm64.deb

.PHONY: upload-deb-package.amd64
upload-deb-package.arm64:
	curl --form uploadfile='@./sandbox/starlight-snapshotter_$(VERSIONNUMBER)_arm64.deb' $(UPLOAD_URL)

######################################################################
.PHONY: docker-buildx-multi-arch
docker-buildx-multi-arch:
	docker buildx build --platform linux/amd64 --build-arg ARCH=amd64 --build-arg UPLOAD_URL=$(UPLOAD_URL)  -f ./demo/deb-package/Dockerfile .
	docker buildx build --platform linux/arm/v7 --build-arg ARCH=armv6l --build-arg UPLOAD_URL=$(UPLOAD_URL)  -f ./demo/deb-package/Dockerfile .
	docker buildx build --platform linux/arm64/v8 --build-arg ARCH=arm64 --build-arg UPLOAD_URL=$(UPLOAD_URL)  -f ./demo/deb-package/Dockerfile .

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
	#service_file=`cat './demo/deb-package/debian/starlight-snapshotter.service'`; \
	#echo `subst "STARLIGHT_PROXY",$(proxy_address),$(service_file)`; \
	#cp ./demo/deb-package/debian/starlight-snapshotter.service /lib/systemd/system/
	#systemctl daemon-reload

docker-image:
	docker build