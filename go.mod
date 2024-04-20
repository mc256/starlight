module github.com/mc256/starlight

go 1.20

require (
	github.com/containerd/containerd v1.6.26
	github.com/containerd/continuity v0.3.0
	github.com/google/go-containerregistry v0.5.1
	github.com/google/uuid v1.3.0

	// resolved overlayfs OICTL issue in https://github.com/hanwen/go-fuse/pull/408
	// tested on Kernel 5.15.0-52-generic
	github.com/hanwen/go-fuse/v2 v2.1.1-0.20221003202731-4c25c9c1eece
	github.com/lib/pq v1.10.6
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc2.0.20221005185240-3a7f492d3f1b
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3

	// newer version causes issue #24
	github.com/urfave/cli/v2 v2.3.0
	go.etcd.io/bbolt v1.3.7
	golang.org/x/net v0.23.0 // indirect
	golang.org/x/sync v0.3.0
	golang.org/x/sys v0.18.0
	google.golang.org/grpc v1.58.3
)

require (
	github.com/pelletier/go-toml v1.9.5
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Microsoft/hcsshim v0.9.10 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/containerd/fifo v1.0.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.4.1 // indirect
	github.com/containerd/ttrpc v1.1.2 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/docker/cli v20.10.17+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker v24.0.9+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.6.3 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/joho/godotenv v1.5.1
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/sys/signal v0.7.0 // indirect
	github.com/opencontainers/runc v1.1.12 // indirect
	github.com/opencontainers/selinux v1.10.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vbauerster/mpb/v8 v8.4.0
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/containerd/containerd v1.6.18 => github.com/containerd/containerd v1.6.26
