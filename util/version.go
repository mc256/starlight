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

package util

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

var Version = "0.9.0"

func VersionAction(context *cli.Context) error {
	fmt.Printf("starlight version %s", Version)
	return nil
}

func VersionCommand() *cli.Command {
	cmd := cli.Command{
		Name:   "version",
		Usage:  "Show version information",
		Action: VersionAction,
	}
	return &cmd
}
