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
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/mc256/starlight/client"
	"github.com/mc256/starlight/util"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, c.App.Version)
	}
}

func main() {
	app := New()
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "starlight-daemon: \n%v\n", err)
		os.Exit(1)
	}
}

func New() *cli.App {
	app := cli.NewApp()
	cfg := client.NewConfig()

	app.Name = "starlight-daemon"
	app.Version = util.Version
	app.Usage = `Daemon for faster container-based application deployment. 

This is a plugin that can be used with containerd to accelerate container-based application deployments.
To enable the plugin, please add plugin configurations to containerd config.toml. You can also verify the plugin status by
running "ctr plugins ls". For more information, please refer to the README.md file in the project repository.
https://github.com/mc256/starlight

*CLI options will override values in the config file if specified.`
	app.Description = fmt.Sprintf("\n%s\n", app.Usage)

	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			DefaultText: "/etc/starlight/daemon_config.json",
			Aliases:     []string{"c"},
			EnvVars:     []string{"STARLIGHT_DAEMON_CONFIG"},
			Usage:       "json configuration file. CLI parameter will override values in the config file if specified",
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
			Name:        "metadata",
			DefaultText: cfg.Metadata,
			Aliases:     []string{"m"},
			Usage:       "path to store image metadata",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "socket",
			DefaultText: cfg.Socket,
			Usage:       "gRPC socket address",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "default",
			DefaultText: cfg.DefaultProxy,
			Aliases:     []string{"d"},
			Usage:       "name of the default proxy",
		},
		&cli.StringFlag{
			Name:        "fs-root",
			DefaultText: cfg.FileSystemRoot,
			Aliases:     []string{"fs"},
			Usage:       "path to store uncompress image layers",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "id",
			DefaultText: cfg.ClientId,
			Usage:       "identifier for the client",
			Required:    false,
		},
		// ----
		&cli.StringSliceFlag{
			Name:        "proxy",
			Aliases:     []string{"p"},
			Usage:       "proxy of the configuration use comma (',') to separate components, and use another tag for other proxies, name,protocol,address,username,password",
			Required:    false,
			DefaultText: "starlight-shared,https,starlight.yuri.moe,,",
		},
	}
	app.Action = func(c *cli.Context) error {
		return DefaultAction(c, cfg)
	}

	return app
}

func DefaultAction(context *cli.Context, cfg *client.Configuration) (err error) {
	var (
		p  string
		ne bool
	)

	config := context.String("config")
	cfg, p, ne, err = client.LoadConfig(config)

	if l := context.String("log-level"); l != "" {
		cfg.LogLevel = l
	}
	c := util.ConfigLoggerWithLevel(cfg.LogLevel)
	log.G(c).
		WithField("version", util.Version).
		Info("starlight-daemon")

	if err != nil {
		log.G(c).WithFields(logrus.Fields{
			"log":  cfg.LogLevel,
			"path": p,
			"new":  ne,
		}).
			WithError(err).
			Fatal("failed to load configuration")
	} else {
		log.G(c).WithFields(logrus.Fields{
			"log":  cfg.LogLevel,
			"path": p,
			"new":  ne,
		}).
			Info("loaded configuration")
	}

	if id := context.String("id"); id != "" {
		cfg.ClientId = id
	}
	if m := context.String("metadata"); m != "" {
		cfg.Metadata = m
	}
	if s := context.String("socket"); s != "" {
		cfg.Socket = s
	}
	if r := context.String("fs-root"); r != "" {
		cfg.FileSystemRoot = r
	}
	if d := context.String("default"); d != "" {
		cfg.DefaultProxy = d
	}
	parr := context.StringSlice("proxy")
	if len(parr) != 0 {
		for _, v := range parr {
			if k, vv, err := client.ParseProxyStrings(v); err != nil {
				log.G(c).
					WithError(err).
					Error("failed to parse proxy flag")
			} else {
				cfg.Proxies[k] = vv
			}
		}
	}

	var slc *client.Client

	slc, err = client.NewClient(c, cfg)
	if err != nil {
		log.G(c).
			WithError(err).
			Fatal("failed to create starlight daemon client")
		os.Exit(1)
		return
	}

	err = slc.StartSnapshotterService()
	if err != nil {
		log.G(c).
			WithError(err).
			Fatal("failed to start snapshotter service")
		os.Exit(1)
		return
	}

	wait := make(chan interface{})
	si := make(chan os.Signal, 1)
	signal.Notify(si, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		<-si
		close(wait)
		slc.Close()
	}()
	<-wait
	return nil
}
