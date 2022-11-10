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
	cmdAddProxy "github.com/mc256/starlight/cmd/ctr-starlight/addproxy"
	cmdConvert "github.com/mc256/starlight/cmd/ctr-starlight/convert"
	cmdNotify "github.com/mc256/starlight/cmd/ctr-starlight/notify"
	cmdOptimizer "github.com/mc256/starlight/cmd/ctr-starlight/optimizer"
	cmdPing "github.com/mc256/starlight/cmd/ctr-starlight/ping"
	cmdPull "github.com/mc256/starlight/cmd/ctr-starlight/pull"
	cmdReport "github.com/mc256/starlight/cmd/ctr-starlight/report"
	cmdVersion "github.com/mc256/starlight/cmd/ctr-starlight/version"
	"os"

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
	app.Usage = `CLI tool for starlight daemon.

This is a CLI tool that controls starlight-daemon. 
Please make sure that starlight-daemon is running before using this tool.
For more information, please refer to the README.md file in the project repository.
https://github.com/mc256/starlight
`
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
			Aliases:     []string{"a", "addr"},
			Value:       "unix:////run/starlight/starlight-daemon.sock",
			DefaultText: "unix:////run/starlight/starlight-daemon.sock",
			EnvVars:     []string{"STARLIGHT_ADDRESS"},
			Usage:       "address to connect to starlight-daemon",
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
		cmdVersion.Command(),   // 1. confirm the version of starlight-daemon
		cmdAddProxy.Command(),  // 2. add starlight proxy to the daemon
		cmdPing.Command(),      // 3. ping the starlight proxy to see if it is alive
		cmdConvert.Command(),   // 3. convert docker image to starlight image
		cmdNotify.Command(),    // 4. notify the proxy that the starlight image is available
		cmdOptimizer.Command(), // 5. turn on/off filesystem traces
		cmdReport.Command(),    // 6. upload filesystem traces to starlight proxy
		cmdPull.Command(),      // 7. pull starlight image
	})

	return app
}
