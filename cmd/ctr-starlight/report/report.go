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

package report

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Action(c *cli.Context) error {
	fmt.Println(c.String("path"))

	return nil
}

func Command() *cli.Command {
	cmd := cli.Command{
		Name:  "report",
		Usage: "Upload data collected by the optimizer back to Starlight Proxy to speed up other similar deployment",
		Action: func(c *cli.Context) error {
			return Action(c)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "path",
				Usage:       "path the the optimizer logs",
				Value:       "/tmp",
				DefaultText: "/tmp",
				Required:    false,
			},
		},
		ArgsUsage: "",
	}
	return &cmd
}
