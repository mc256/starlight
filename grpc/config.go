package grpc

import (
	"context"
	"encoding/json"
	"github.com/containerd/containerd/log"
	"github.com/google/uuid"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path"
)

type ProxyConfig struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`

	// Auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type Configuration struct {
	ctx context.Context

	LogLevel string `json:"log_level"`
	ClientId string `json:"id"`

	// path to database
	Metadata string `json:"metadata"`

	// socket address
	Socket string `json:"socket"`

	// registry + proxy
	DefaultProxy   string `json:"default_proxy"`
	FileSystemRoot string `json:"fs_root"`

	Proxies map[string]ProxyConfig `json:"configs"`
}

func LoadConfig(ctx context.Context) (c *Configuration) {
	c = newConfig(ctx)

	etcPath := util.GetEtcConfigPath()
	if err := os.MkdirAll(etcPath, 0775); err != nil {
		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot create config folder"))
		return
	}

	configPath := path.Join(etcPath, "snapshotter_config.json")
	log.G(c.ctx).WithFields(logrus.Fields{
		"path": configPath,
	}).Info("loading config")

	b, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.G(c.ctx).Warn(errors.Wrapf(err, "cannot open config file"))

		buf, _ := json.Marshal(c)
		err := ioutil.WriteFile(configPath, buf, 0644)
		if err == nil {
			log.G(c.ctx).Info("created new config file.")
			return
		}

		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot create config file"))
		return
	}

	if err = json.Unmarshal(b, c); err != nil {
		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot parse config file"))
		return
	}

	return
}

func (c *Configuration) SaveConfig() error {
	etcPath := util.GetEtcConfigPath()
	if err := os.MkdirAll(etcPath, 0775); err != nil {
		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot create config folder"))
		return err
	}

	configPath := path.Join(etcPath, "snapshotter_config.json")
	buf, _ := json.MarshalIndent(c, " ", " ")
	err := ioutil.WriteFile(configPath, buf, 0644)
	if err == nil {
		log.G(c.ctx).Info("created new config file.")
		return err
	}

	return nil
}

func newConfig(ctx context.Context) *Configuration {
	uuid.EnableRandPool()
	return &Configuration{
		ctx: ctx,

		LogLevel:       "debug",
		Metadata:       "/var/lib/starlight-grpc/metadata.db",
		Socket:         "/run/starlight-grpc/starlight-snapshotter.socket",
		DefaultProxy:   "starlight-shared",
		FileSystemRoot: "/var/lib/starlight-grpc",
		ClientId:       uuid.New().String(),

		Proxies: map[string]ProxyConfig{
			"starlight-shared": {
				Protocol: "https",
				Address:  "starlight.yuri.moe",

				Username: "",
				Password: "",
			},
		},
	}
}
