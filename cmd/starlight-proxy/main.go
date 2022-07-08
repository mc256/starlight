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
	"github.com/mc256/starlight/proxy"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
	"os"
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
		_, _ = fmt.Fprintf(os.Stderr, "starlight-proxy: %v\n", err)
		os.Exit(1)
	}
}

func New() *cli.App {
	app := cli.NewApp()

	app.Name = "starlight-proxy"
	app.Version = util.Version
	app.Usage = `Starlight Proxy accelerates container deployments`
	app.Description = fmt.Sprintf("\n%s\n", app.Usage)

	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "registry",
			Usage: "Registry Address",
			EnvVars: []string{
				"REGISTRY",
			},
			Required: true,
		},
		&cli.StringFlag{
			Name:        "log-level",
			DefaultText: "info",
			Usage:       "Choose one log level (fatal, error, warning, info, debug, trace)",
			EnvVars: []string{
				"LOGLEVEL",
			},
			Required: false,
		},
	}
	app.Commands = append([]*cli.Command{
		util.VersionCommand(),
	})
	app.Action = DefaultAction

	return app

}

func DefaultAction(context *cli.Context) error {

	logLevel := "info"
	if l := context.String("log-level"); l != "" {
		logLevel = l
	}

	var registry string
	if registry = context.String("registry"); registry == "" {
		fmt.Println("registry cannot be empty!")
		return nil
	}

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)

	_ = proxy.NewServer(registry, logLevel, httpServerExitDone)

	wait := make(chan interface{})
	<-wait
	return nil
}
