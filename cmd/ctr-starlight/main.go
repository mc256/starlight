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
	"os"

	cmdConvert "github.com/mc256/starlight/cmd/ctr-starlight/convert"
	cmdCreate "github.com/mc256/starlight/cmd/ctr-starlight/create"
	cmdNotify "github.com/mc256/starlight/cmd/ctr-starlight/notify"
	cmdPull "github.com/mc256/starlight/cmd/ctr-starlight/pull"
	cmdReport "github.com/mc256/starlight/cmd/ctr-starlight/report"

	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, c.App.Version)
	}
}

func main() {
	app := NewApp()
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "ctr-starlight: \n%v\n", err)
		os.Exit(1)
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()

	app.Name = "ctr-starlight"
	app.Version = util.Version
	app.Usage = `starlight container deployment with remote delta image`
	app.Description = fmt.Sprintf("\n%s\n", app.Usage)

	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "namespace",
			Aliases:     []string{"n"},
			Value:       "default",
			DefaultText: "default",
			EnvVars:     []string{"CONTAINERD_NAMESPACE"},
			Usage:       "namespace to use with commands",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "address",
			Aliases:     []string{"a"},
			Value:       "/run/containerd/containerd.sock",
			DefaultText: "/run/containerd/containerd.sock",
			EnvVars:     []string{"CONTAINERD_ADDRESS"},
			Usage:       "address for containerd's GRPC server",
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "log-level",
			Usage:       "log level for this command line tool",
			Value:       "info",
			DefaultText: "info",
			Required:    false,
		},
	}
	app.Commands = append([]*cli.Command{
		util.VersionCommand(),
		cmdPull.Command(),
		cmdCreate.Command(),
		cmdReport.Command(),
		cmdConvert.Command(),
		cmdNotify.Command(),
	})

	return app
}
