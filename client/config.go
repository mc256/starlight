package client

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type ProxyConfig struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`

	// Auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type Configuration struct {
	LogLevel string `json:"log_level"`
	ClientId string `json:"id"`

	// path to database
	Metadata string `json:"metadata"`

	// socket address
	Socket     string `json:"socket"`
	DaemonType string `json:"daemon_type"`
	Daemon     string `json:"daemon"`
	Containerd string `json:"containerd"`
	Namespace  string `json:"default_namespace"`
	TracesDir  string `json:"traces_dir"`

	// registry + proxy
	DefaultProxy   string `json:"default_proxy"`
	FileSystemRoot string `json:"fs_root"`

	Proxies map[string]*ProxyConfig `json:"configs"`
}

func (c *Configuration) getProxy(name string) (pc *ProxyConfig, key string) {
	if name == "" {
		name = c.DefaultProxy
	}
	if p, has := c.Proxies[name]; has {
		return p, name
	}
	name = c.DefaultProxy
	return c.Proxies[name], name
}

func ParseProxyStrings(v string) (name string, c *ProxyConfig, err error) {
	sp := strings.Split(v, ",")
	if len(sp) < 3 {
		return "", nil, fmt.Errorf("failed to parse '%s'", v)
	}

	c = &ProxyConfig{
		Protocol: sp[1],
		Address:  sp[2],
	}
	if len(sp) == 4 {
		c.Username = sp[3]
	}
	if len(sp) == 5 {
		c.Password = sp[4]
	}
	return sp[0], c, nil
}

func LoadConfig(cfgPath string) (c *Configuration, p string, n bool, error error) {
	c = NewConfig()

	if cfgPath == "" {
		etcPath := util.GetEtcConfigPath()
		if err := os.MkdirAll(etcPath, 0775); err != nil {
			error = errors.Wrapf(err, "cannot create config folder")
			return
		}

		p = path.Join(etcPath, "starlight-daemon.json")

	} else {
		p = cfgPath
	}

	b, err := ioutil.ReadFile(p)
	n = false
	if err != nil {
		n = true

		buf, _ := json.MarshalIndent(c, " ", " ")
		if err = ioutil.WriteFile(p, buf, 0644); err == nil {
			return
		} else {
			error = errors.Wrapf(err, "cannot create config file")
		}
	}

	if err = json.Unmarshal(b, c); err != nil {
		error = errors.Wrapf(err, "cannot parse config file")
	}
	return
}

func (c *Configuration) SaveConfig() error {
	etcPath := util.GetEtcConfigPath()
	if err := os.MkdirAll(etcPath, 0775); err != nil {
		return errors.Wrapf(err, "cannot create config folder")
	}

	p := path.Join(etcPath, "starlight-daemon.json")
	buf, _ := json.MarshalIndent(c, " ", " ")
	err := ioutil.WriteFile(p, buf, 0644)
	if err == nil {
		return nil
	}

	return nil
}

func NewConfig() *Configuration {
	uuid.EnableRandPool()
	return &Configuration{
		LogLevel:       "info",
		Metadata:       "/var/lib/starlight/metadata.db",
		Socket:         "/run/starlight/starlight-snapshotter.sock",
		DaemonType:     "unix",
		Daemon:         "/run/starlight/starlight-daemon.sock",
		Containerd:     "/run/containerd/containerd.sock",
		DefaultProxy:   "starlight-shared",
		FileSystemRoot: "/var/lib/starlight",
		TracesDir:      "/var/lib/starlight/traces",
		Namespace:      "default",
		ClientId:       uuid.New().String(),

		Proxies: map[string]*ProxyConfig{
			"starlight-shared": {
				Protocol: "https",
				Address:  "starlight.yuri.moe",

				Username: "",
				Password: "",
			},
		},
	}
}
