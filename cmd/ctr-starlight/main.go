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

	cmdAddProxy "github.com/mc256/starlight/cmd/ctr-starlight/addproxy"
	cmdConvert "github.com/mc256/starlight/cmd/ctr-starlight/convert"
	cmdListProxy "github.com/mc256/starlight/cmd/ctr-starlight/listproxy"
	cmdNotify "github.com/mc256/starlight/cmd/ctr-starlight/notify"
	cmdOptimizer "github.com/mc256/starlight/cmd/ctr-starlight/optimizer"
	cmdPing "github.com/mc256/starlight/cmd/ctr-starlight/ping"
	cmdPull "github.com/mc256/starlight/cmd/ctr-starlight/pull"
	cmdReport "github.com/mc256/starlight/cmd/ctr-starlight/report"
	cmdVersion "github.com/mc256/starlight/cmd/ctr-starlight/version"

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
		_, _ = fmt.Fprintf(os.Stderr, "ctr-starlight: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
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
			Usage:       "namespace to use with commands (if using kubernetes, please specify `k8s.io`)",
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
			Aliases:     []string{"l"},
			Usage:       "log level for this command line tool",
			Value:       "info",
			DefaultText: "info",
			Required:    false,
		},
	}
	app.Commands = []*cli.Command{
		cmdVersion.Command(),   // 1. confirm the version of starlight-daemon
		cmdAddProxy.Command(),  // 2. add starlight proxy to the daemon
		cmdListProxy.Command(), // 3. list proxy profiles in daemon
		cmdPing.Command(),      // 4. ping the starlight proxy to see if it is alive
		cmdConvert.Command(),   // 5. convert docker image to starlight image
		cmdNotify.Command(),    // 6. notify the proxy that the starlight image is available
		cmdOptimizer.Command(), // 7. turn on/off filesystem traces
		cmdReport.Command(),    // 8. upload filesystem traces to starlight proxy
		cmdPull.Command(),      // 9. pull starlight image
	}

	return app
}
