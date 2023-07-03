/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"encoding/json"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/mc256/starlight/util"
	"github.com/pkg/errors"
)

type Configuration struct {
	// server
	ListenPort    int    `json:"port"`
	ListenAddress string `json:"address"`
	LogLevel      string `json:"log_level"`

	// database
	PostgresConnectionString string `json:"postgres"`

	// registry
	DefaultRegistry      string   `json:"default_registry"`
	DefaultRegistryAlias []string `json:"default_registry_alias"`

	// goharbor hook
	//EnableHarborScanner bool   `json:"harbor"`
	//HarborApiKey        string `json:"harbor_apikey"`

	// layer cache timeout (second)
	CacheTimeout int `json:"cache_timeout"`
}

func LoadConfig(cfgPath string) (c *Configuration, p string, n bool, error error) {
	c = NewConfig()

	if cfgPath == "" {
		etcPath := util.GetEtcConfigPath()
		if err := os.MkdirAll(etcPath, 0775); err != nil {
			error = errors.Wrapf(err, "cannot create config folder")
			return
		}

		p = path.Join(etcPath, "starlight-proxy.json")
	} else {
		p = cfgPath
	}

	b, err := os.ReadFile(p)
	n = false
	if err != nil {
		n = true

		buf, _ := json.Marshal(c)

		if err = os.WriteFile(p, buf, 0644); err == nil {
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

	p := path.Join(etcPath, "starlight-proxy.json")
	buf, _ := json.MarshalIndent(c, " ", " ")
	err := os.WriteFile(p, buf, 0644)
	if err == nil {
		return nil
	}

	return nil
}

func NewConfig() *Configuration {
	uuid.EnableRandPool()
	return &Configuration{
		ListenPort:               8090,
		ListenAddress:            "0.0.0.0",
		LogLevel:                 "info",
		PostgresConnectionString: "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable",
		DefaultRegistry:          "127.0.0.1:9000",
		DefaultRegistryAlias: []string{
			"localhost:9000",
		},

		//EnableHarborScanner: false,
		//HarborApiKey:        uuid.New().String(),

		CacheTimeout: 3600,
	}
}
