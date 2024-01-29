module github.com/mc256/starlight

go 1.20

require (
	github.com/containerd/containerd v1.7.12
	github.com/containerd/continuity v0.4.3
	github.com/google/go-containerregistry v0.18.0
	github.com/google/uuid v1.6.0

	// resolved overlayfs OICTL issue in https://github.com/hanwen/go-fuse/pull/408
	// tested on Kernel 5.15.0-52-generic
	github.com/hanwen/go-fuse/v2 v2.4.2
	github.com/lib/pq v1.10.9
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc6
	github.com/opencontainers/runtime-spec v1.1.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3

	// newer version causes issue #24
	github.com/urfave/cli/v2 v2.27.1
	go.etcd.io/bbolt v1.3.8
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.16.0
	google.golang.org/grpc v1.61.0
)

require (
	github.com/pelletier/go-toml v1.9.5
	google.golang.org/protobuf v1.32.0
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230811130428-ced1acdcaa24 // indirect
	github.com/AdamKorcz/go-118-fuzz-build v0.0.0-20231105174938-2b5cbb29f3e2 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/rivo/uniseg v0.4.6 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/xrash/smetrics v0.0.0-20231213231151-1d8dd44e695e // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.47.0 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.22.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	google.golang.org/genproto v0.0.0-20240125205218-1f4bbc51befe // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240125205218-1f4bbc51befe // indirect
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Microsoft/hcsshim v0.11.4 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/containerd/fifo v1.1.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.15.1 // indirect
	github.com/containerd/ttrpc v1.2.2 // indirect
	github.com/containerd/typeurl v1.0.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/docker/cli v25.0.1+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v25.0.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.1 // indirect
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/joho/godotenv v1.5.1
	github.com/klauspost/compress v1.17.5 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/sys/signal v0.7.0 // indirect
	github.com/opencontainers/runc v1.1.11 // indirect
	github.com/opencontainers/selinux v1.11.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/vbauerster/mpb/v8 v8.7.2
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)
