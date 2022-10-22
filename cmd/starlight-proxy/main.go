/*
   Copyright The starlight Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   file created by maverick in 2021
*/

package main

import (
	"context"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
	"os"
	"regexp"
	"sync"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, c.App.Version)
	}
}

func main() {
	app := New()
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "starlight-proxy: \n%v\n", err)
		os.Exit(1)
	}
}

func ProtectPassword(c string) string {
	p := regexp.MustCompile(`:(.*)@`)
	return p.ReplaceAllString(c, ":********@")
}

func New() *cli.App {
	app := cli.NewApp()
	cfg := proxy.LoadConfig(context.TODO())

	app.Name = "starlight-proxy"
	app.Version = util.Version
	app.Usage = `Starlight Proxy accelerates container deployments. 

This is a proxy server on the cloud side mediates between Starlight workers and any standard registry server. For more
information about Starlight, please visit our repository at https://github.com/mc256/starlight

*CLI options will override values in the config file if specified.
`
	app.Description = fmt.Sprintf("\n%s\n", app.Usage)

	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			DefaultText: "/etc/starlight/proxy_config.json",
			Aliases:     []string{"c"},
			EnvVars:     []string{"STARLIGHT_PROXY_CONFIG"},
			Usage:       "json configuration file.",
			Required:    false,
		},
		// ----
		&cli.StringFlag{
			Name:        "host",
			DefaultText: cfg.ListenAddress,
			Usage:       "host",
			Required:    false,
		},
		&cli.IntFlag{
			Name:        "port",
			DefaultText: fmt.Sprintf("%d", cfg.ListenPort),
			Aliases:     []string{"p"},
			Usage:       "proxy port",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "log-level",
			DefaultText: cfg.LogLevel,
			Usage:       "Choose one log level (fatal, error, warning, info, debug, trace)",
			Required:    false,
		},
		// ----
		&cli.StringFlag{
			Name:        "postgres",
			DefaultText: ProtectPassword(cfg.PostgresConnectionString),
			Usage:       "use PostgreSQL database backend for storing TOCs",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "postgres-schema",
			DefaultText: cfg.PostgresDBSchema,
			Usage:       "the schema to hold tables",
			Required:    false,
		},
		// ----
		&cli.StringFlag{
			Name:        "registry",
			DefaultText: cfg.DefaultRegistry,
			Usage:       "Default container registry",
			Required:    false,
		},
		// ----
		&cli.BoolFlag{
			Name:        "goharbor",
			DefaultText: fmt.Sprintf("%v", cfg.EnableHarborScanner),
			Usage:       "integrate goharbor and enable auto container image conversion",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "goharbor-apikey",
			DefaultText: cfg.HarborApiKey,
			Usage:       "api key for verify the scan requests",
			Required:    false,
		},
	}
	app.Action = func(c *cli.Context) error {
		return DefaultAction(c, cfg)
	}

	return app
}

func DefaultAction(context *cli.Context, cfg *proxy.Configuration) error {
	if l := context.String("log-level"); l != "" {
		cfg.LogLevel = l
	}
	c := util.ConfigLoggerWithLevel(cfg.LogLevel)

	if p := context.Int("port"); p != 0 {
		cfg.ListenPort = p
	}
	if h := context.String("host"); h != "" {
		cfg.ListenAddress = h
	}
	log.G(c).Infof("Starlight Proxy listen on %s:%d", cfg.ListenAddress, cfg.ListenPort)

	if pc := context.String("postgres"); pc != "" {
		cfg.PostgresConnectionString = pc
	}
	if ps := context.String("postgres-schema"); ps != "" {
		cfg.PostgresDBSchema = ps
	}

	if r := context.String("registry"); r != "" {
		cfg.DefaultRegistry = r
	}
	log.G(c).Infof("Backend Registry: %s", cfg.DefaultRegistry)

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	_ = proxy.NewServer(c, httpServerExitDone, cfg)

	wait := make(chan interface{})
	<-wait
	return nil
}
