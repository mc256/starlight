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
	runCommand "github.com/mc256/starlight/cmd/starlight-grpc/run"
	"github.com/mc256/starlight/util"
	"github.com/urfave/cli/v2"
	"os"
)

func init() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(c.App.Name, c.App.Version)
	}
}

func main() {
	app := New()
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "starlight-grpc: \n%v\n", err)
		os.Exit(1)
	}
}

func New() *cli.App {
	app := cli.NewApp()

	app.Name = "starlight-grpc"
	app.Version = util.Version
	app.Usage = `gRPC snapshotter plugin for faster container-based application deployment`
	app.Description = fmt.Sprintf("\n%s\n", app.Usage)

	app.EnableBashCompletion = true
	app.Flags = []cli.Flag{}
	app.Commands = append([]*cli.Command{
		util.VersionCommand(),
		runCommand.RunCommand(),
	})

	return app
}
