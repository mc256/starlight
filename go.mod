module github.com/mc256/starlight

go 1.15

require (
	github.com/containerd/containerd v1.4.3
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-cmp v0.5.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mc256/stargz-snapshotter/estargz v0.0.0-00010101000000-000000000000
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/prometheus/client_golang v1.7.1 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/stretchr/testify v1.5.1 // indirect
	go.etcd.io/bbolt v1.3.5
	golang.org/x/sys v0.0.0-20201202213521-69691e467435 // indirect
	google.golang.org/protobuf v1.24.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v2 v2.3.0 // indirect
	gotest.tools/v3 v3.0.3 // indirect
)

replace github.com/mc256/stargz-snapshotter/estargz v0.0.0-00010101000000-000000000000 => github.com/mc256/stargz-snapshotter/estargz v0.0.0-20210121032037-7677345cbbc6
