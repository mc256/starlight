/*
   file created by Junlin Chen in 2022

*/

package proxy

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

type ProxyConfiguration struct {
	ctx context.Context

	// server
	ListenPort    int    `json:"port"`
	ListenAddress string `json:"address"`
	LogLevel      string `json:"log_level"`

	// database
	PostgresConnectionString string `json:"postgres"`
	PostgresDBSchema         string `json:"postgres_schema"`

	// registry
	DefaultRegistry string `json:"default_registry"`

	// goharbor hook
	EnableHarborScanner bool   `json:"harbor"`
	HarborApiKey        string `json:"harbor_apikey"`

	// layer cache timeout (second)
	CacheTimeout int `json:"cache_timeout"`
}

func LoadConfig(ctx context.Context) (c *ProxyConfiguration) {
	c = newConfig(ctx)

	etcPath := util.GetEtcConfigPath()
	if err := os.MkdirAll(etcPath, 0775); err != nil {
		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot create config folder"))
		return
	}

	configPath := path.Join(etcPath, "proxy_config.json")
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

func (c *ProxyConfiguration) SaveConfig() error {
	etcPath := util.GetEtcConfigPath()
	if err := os.MkdirAll(etcPath, 0775); err != nil {
		log.G(c.ctx).Fatal(errors.Wrapf(err, "cannot create config folder"))
		return err
	}

	configPath := path.Join(etcPath, "proxy_config.json")
	buf, _ := json.MarshalIndent(c, " ", " ")
	err := ioutil.WriteFile(configPath, buf, 0644)
	if err == nil {
		log.G(c.ctx).Info("created new config file.")
		return err
	}

	return nil
}

func newConfig(ctx context.Context) *ProxyConfiguration {
	uuid.EnableRandPool()
	return &ProxyConfiguration{
		ctx:                      ctx,
		ListenPort:               8090,
		ListenAddress:            "0.0.0.0",
		LogLevel:                 "info",
		PostgresConnectionString: "postgres://starlight:password@localhost/starlight?sslmode=disable",
		PostgresDBSchema:         "starlight",
		DefaultRegistry:          "127.0.0.1:9000",
		EnableHarborScanner:      false,
		HarborApiKey:             uuid.New().String(),
		CacheTimeout:             3600,
	}
}
